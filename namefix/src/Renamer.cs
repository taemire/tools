// namefix - 파일 시스템 이름 변경
//
// 요청(WorkItem)을 실제 대상 경로 목록으로 펼친 뒤, 깊은 경로부터 이름을 바꿉니다.
// 안전 원칙:
//   - 새 이름을 원본이 아닌 다른 디렉터리 항목이 쓰고 있으면 덮어쓰지 않고 충돌로 보고합니다(하드 링크 포함).
//   - 실패한 항목은 원래 이름을 유지하며, 나머지 항목은 계속 처리합니다.
//   - 재분석 지점(심볼릭 링크·정션) 폴더 안으로는 들어가지 않습니다. 시작 폴더가 링크여도 같습니다.
// 멱등성과 요청 항목 확인:
//   - 이름 변환이 멱등이므로 같은 항목을 다시 처리해도 이름이 더 바뀌지 않습니다.
//   - 요청 시점에 원본의 식별자(볼륨·파일 ID)를 기록합니다. 처리 시점에 요청 경로가 없거나 다른 항목으로 바뀌었으면,
//     식별자로 원본의 현재 경로를 찾아 처리합니다(원본 또는 상위 폴더의 이름이 먼저 정리된 경우 포함).
//     찾지 못하면 오류로 보고하며, 요청 경로에 새로 생긴 다른 항목이나 이름만 대응하는 다른 항목은 처리하지 않습니다.
//     다중 실행 집계에서 수집자가 처리 도중 종료되어 보낸 쪽이 다시 처리하는 경우가 이 성질로 무해해집니다.
using System;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Linq;

namespace NameFix
{
    /// <summary>항목 한 개의 처리 결과 종류입니다.</summary>
    internal enum ItemStatus
    {
        /// <summary>이름을 바꿨습니다.</summary>
        Renamed,

        /// <summary>미리 보기(--dry-run)에서 바뀔 예정인 항목입니다.</summary>
        Planned,

        /// <summary>이미 정상 이름이라 바꾸지 않았습니다.</summary>
        Unchanged,

        /// <summary>바꿀 이름의 다른 항목이 이미 있어 바꾸지 않았습니다.</summary>
        Conflict,

        /// <summary>오류로 바꾸지 못했습니다.</summary>
        Error
    }

    /// <summary>항목 한 개의 처리 결과입니다.</summary>
    internal sealed class ItemResult
    {
        public string OldPath;
        public string NewPath;
        public ItemStatus Status;
        public string Message;

        /// <summary>
        /// 요청 항목이 요청 이후 다른 실행에 의해 이미 정리되어 현재 경로에 있는 경우입니다(Unchanged의 일종).
        /// 항목 식별자가 요청 시점과 같을 때만 이렇게 판정합니다.
        /// </summary>
        public bool AlreadyNormalized;

        /// <summary>이 결과를 낳은 요청의 출처(요청 ID) 목록입니다. 다중 실행 집계에서 요청별 성공·실패를 돌려줄 때 씁니다.</summary>
        public readonly List<string> Origins = new List<string>();

        /// <summary>충돌 또는 오류인지 여부입니다.</summary>
        public bool IsProblem
        {
            get { return Status == ItemStatus.Conflict || Status == ItemStatus.Error; }
        }

        /// <summary>오류 결과를 만듭니다.</summary>
        public static ItemResult Failure(string path, string message, string origin)
        {
            ItemResult result = new ItemResult();
            result.OldPath = path;
            result.Status = ItemStatus.Error;
            result.Message = message;
            result.Origins.Add(origin ?? string.Empty);
            return result;
        }
    }

    /// <summary>처리 결과 집계입니다.</summary>
    internal sealed class Summary
    {
        public int Renamed;
        public int Planned;
        public int Unchanged;
        public int Conflicts;
        public int Errors;

        /// <summary>Unchanged 중 "이미 정리됨"으로 처리한 항목 수입니다.</summary>
        public int AlreadyNormalized;

        public int Total
        {
            get { return Renamed + Planned + Unchanged + Conflicts + Errors; }
        }

        public static Summary From(IEnumerable<ItemResult> results)
        {
            Summary s = new Summary();
            foreach (ItemResult r in results)
            {
                switch (r.Status)
                {
                    case ItemStatus.Renamed: s.Renamed++; break;
                    case ItemStatus.Planned: s.Planned++; break;
                    case ItemStatus.Unchanged:
                        s.Unchanged++;
                        if (r.AlreadyNormalized)
                        {
                            s.AlreadyNormalized++;
                        }
                        break;
                    case ItemStatus.Conflict: s.Conflicts++; break;
                    default: s.Errors++; break;
                }
            }
            return s;
        }
    }

    /// <summary>
    /// 처리 요청 한 건입니다. 경로, 처리 범위, 이름 변환 옵션을 함께 담습니다.
    /// 다중 실행 집계 시 요청마다 옵션이 다를 수 있으므로 옵션을 요청 단위로 보관합니다.
    /// 경로는 요청한 프로세스의 작업 폴더 기준 절대 경로로 확정해 둡니다(Program 참조).
    /// 파일 경로는 범위와 관계없이 항상 자기 자신만 처리합니다.
    /// </summary>
    internal sealed class WorkItem
    {
        /// <summary>대상 경로입니다.</summary>
        public string Path;

        /// <summary>폴더를 지정했을 때 폴더 자신의 이름도 정리할지 여부입니다.</summary>
        public bool IncludeSelf = true;

        /// <summary>폴더 하위 처리 깊이입니다. 0: 하위 없음, 1: 직속 하위만, -1: 모든 하위.</summary>
        public int Depth;

        /// <summary>이 요청에 적용할 이름 변환 옵션입니다.</summary>
        public NormalizeOptions Options = new NormalizeOptions();

        /// <summary>
        /// 요청 시점에 경로의 항목이 있었는지 여부입니다. 없던 경로는 처리 시점에 항목이 생겼어도 처리하지 않습니다.
        /// </summary>
        public bool Existed;

        /// <summary>
        /// 요청 시점의 원본 식별자(볼륨 일련번호-파일 ID)입니다. 원본이 없었거나 식별자를 얻을 수 없으면(파일 ID 미제공) 빈 문자열입니다.
        /// 처리 시점에 경로가 없거나 경로의 항목이 이 식별자와 다르면, 식별자로 원본의 현재 경로를 찾아 처리합니다.
        /// </summary>
        public string Identity = string.Empty;

        /// <summary>
        /// 요청의 출처(요청 ID)입니다. 파이프로 전송하지 않으며, 집계하는 쪽이 요청 ID로 채웁니다.
        /// 명령줄 실행처럼 집계하지 않으면 빈 문자열입니다.
        /// </summary>
        public string Origin = string.Empty;

        // 파이프 전송 형식: "<IncludeSelf 0|1>|<Depth>|<UrlDecode 0|1>|<Nfc 0|1>|<Existed 0|1>|<Identity>|<Path>"
        // Windows 경로와 식별자에는 '|'와 줄바꿈이 올 수 없으므로 구분자로 안전합니다.
        public string Serialize()
        {
            return (IncludeSelf ? "1" : "0") + "|" +
                   Depth.ToString(CultureInfo.InvariantCulture) + "|" +
                   (Options.UrlDecode ? "1" : "0") + "|" +
                   (Options.Nfc ? "1" : "0") + "|" +
                   (Existed ? "1" : "0") + "|" +
                   (Identity ?? string.Empty) + "|" +
                   Path;
        }

        public static string SerializeMany(IEnumerable<WorkItem> items)
        {
            return string.Join("\n", items.Select(w => w.Serialize()).ToArray());
        }

        /// <summary>전송된 줄 목록을 요청 목록으로 되돌립니다. 형식이 맞지 않는 줄은 버립니다.</summary>
        public static List<WorkItem> ParseMany(IEnumerable<string> lines)
        {
            List<WorkItem> list = new List<WorkItem>();
            foreach (string line in lines)
            {
                string[] parts = line.Split(new[] { '|' }, 7);
                int depth;
                if (parts.Length != 7 || parts[6].Length == 0 ||
                    !int.TryParse(parts[1], NumberStyles.AllowLeadingSign, CultureInfo.InvariantCulture, out depth) ||
                    depth < -1)
                {
                    continue;
                }
                WorkItem w = new WorkItem();
                w.IncludeSelf = parts[0] == "1";
                w.Depth = depth;
                w.Options.UrlDecode = parts[2] == "1";
                w.Options.Nfc = parts[3] == "1";
                w.Existed = parts[4] == "1";
                w.Identity = parts[5];
                w.Path = parts[6];
                list.Add(w);
            }
            return list;
        }
    }

    /// <summary>요청을 대상 경로로 펼치고 이름을 바꿉니다.</summary>
    internal static class Renamer
    {
        // 대체 탐색에서 확인할 최대 후보 경로 수입니다(경로 단계마다 원래 이름과 정리된 이름 두 갈래).
        private const int MaxResolveCandidates = 256;

        // 대상 경로, 디렉터리 항목 키(TargetSet 참조), 그 경로에 적용할 변환 옵션, 이 경로를 포함한 요청의 출처 목록입니다.
        private sealed class Target
        {
            public string Path;
            public string Key;
            public NormalizeOptions Options;
            public readonly List<string> Origins = new List<string>();
        }

        /// <summary>
        /// 대상 목록입니다. 같은 디렉터리 항목을 가리키는 다른 경로 표기(D:\work와 D:\WORK, 정션 별칭)를 하나로 합칩니다.
        /// 기준은 "링크를 따라간 실제 부모 폴더의 식별자 + 디스크에 저장된 항목 이름"입니다.
        /// 같은 파일의 별개 하드 링크와 대소문자를 구분하는 폴더의 대소문자만 다른 항목은 저장 이름이 다르므로 따로 둡니다.
        /// 부모 폴더 식별자나 저장 이름을 알 수 없으면 경로 문자열(서수 비교)로 구분합니다.
        /// 같은 항목이 여러 요청에 있으면 첫 요청의 경로 표기와 옵션을 따르고, 결과는 그 항목을 포함한 모든 요청에 돌려줍니다.
        /// </summary>
        private sealed class TargetSet
        {
            public readonly List<Target> Items = new List<Target>();
            private readonly Dictionary<string, Target> byKey = new Dictionary<string, Target>(StringComparer.Ordinal);
            private readonly Dictionary<string, string> folderIds = new Dictionary<string, string>(StringComparer.Ordinal);
            private readonly Dictionary<string, string> realFolders = new Dictionary<string, string>(StringComparer.Ordinal);
            private readonly Dictionary<string, string> pathKeys = new Dictionary<string, string>(StringComparer.Ordinal);

            /// <param name="storedName">디렉터리 열거로 얻은 저장 이름입니다. null이면 부모 폴더를 검색해 찾습니다.</param>
            public void Add(string path, string storedName, NormalizeOptions options, string origin)
            {
                string key;
                string actualPath;
                Resolve(path, storedName, out key, out actualPath);
                Target target;
                if (!byKey.TryGetValue(key, out target))
                {
                    target = new Target();
                    // 처리 경로는 "링크를 따라간 실제 부모 경로 + 저장 이름"으로 확정합니다.
                    // - 입력한 표기(예: A%20X.TXT)가 아니라 저장 이름(예: a%20x.txt)을 정리하므로 표기에 따라 결과가 달라지지 않습니다.
                    // - 부모 경로에 정션이 남지 않으므로, 다른 대상(정션 자체 또는 정션이 가리키는 경로의 폴더, 연쇄 정션의 중간
                    //   경유지)의 이름이 바뀌어도 이 경로는 끊기지 않습니다. 이 경로에 영향을 주는 것은 실제 상위 폴더뿐입니다.
                    target.Path = actualPath;
                    target.Key = key;
                    target.Options = options;
                    byKey[key] = target;
                    Items.Add(target);
                }
                if (!target.Origins.Contains(origin))
                {
                    target.Origins.Add(origin);
                }
            }

            /// <summary>
            /// 임의 경로(대상의 상위 경로 구성요소)의 디렉터리 항목 키를 대상과 같은 규칙으로 계산합니다(경로별 캐시).
            /// 처리 순서를 정할 때, 어떤 대상의 경로가 다른 대상 항목을 거쳐 가는지 확인하는 데 씁니다.
            /// </summary>
            public string KeyOfPath(string path)
            {
                string key;
                if (!pathKeys.TryGetValue(path, out key))
                {
                    string actualPath;
                    Resolve(path, null, out key, out actualPath);
                    pathKeys[path] = key;
                }
                return key;
            }

            /// <summary>키가 key인 대상이 있으면 돌려줍니다.</summary>
            public Target FindByKey(string key)
            {
                Target target;
                return byKey.TryGetValue(key, out target) ? target : null;
            }

            // 중복 제거 키와, 부모를 실제 경로로·마지막 이름을 저장 이름으로 바꾼 처리 경로를 정합니다.
            // 마지막 구성요소는 링크를 따라가지 않습니다(대상이 정션이면 링크 항목 자체의 이름을 바꿉니다).
            private void Resolve(string path, string storedName, out string key, out string actualPath)
            {
                key = "path:" + path;
                actualPath = path;
                string parent = System.IO.Path.GetDirectoryName(path);
                string name = System.IO.Path.GetFileName(path);
                if (string.IsNullOrEmpty(parent) || string.IsNullOrEmpty(name))
                {
                    return;
                }
                string realFolder;
                if (!realFolders.TryGetValue(parent, out realFolder))
                {
                    // 실제 경로를 얻지 못하는 파일 시스템에서는 입력한 부모 경로를 그대로 씁니다.
                    realFolder = NativeMethods.TryGetFinalPath(parent) ?? parent;
                    realFolders[parent] = realFolder;
                }
                string stored = storedName ?? FindStoredName(parent, name);
                actualPath = System.IO.Path.Combine(realFolder, stored ?? name);
                if (stored == null)
                {
                    key = "path:" + actualPath;
                    return;
                }
                string folderId;
                if (!folderIds.TryGetValue(parent, out folderId))
                {
                    folderId = NativeMethods.TryGetFileIdentity(parent, true) ?? string.Empty;
                    folderIds[parent] = folderId;
                }
                key = folderId.Length == 0 ? "path:" + actualPath : "id:" + folderId + "|" + stored;
            }
        }

        /// <summary>
        /// 처리 순서를 정합니다. 대상 경로는 수집할 때 "링크를 따라간 실제 부모 경로 + 저장 이름"으로 확정했으므로(TargetSet),
        /// 경로에 정션이 없고 경로를 끊을 수 있는 것은 실제 상위 폴더뿐입니다. 대상 T 경로의 상위 구성요소가 다른 대상 D의 항목이면
        /// D의 이름을 먼저 바꿀 때 T의 경로가 무효가 되므로 T를 D보다 먼저 처리합니다.
        /// 상위 구성요소가 같은 항목인지는 디렉터리 항목 키로 판정합니다(드라이브 루트 바로 아래 항목 포함).
        /// 위 관계로 정해지지 않는 순서는 경로 구성요소 수(루트 제외) 내림차순, 그다음 수집 순서를 따릅니다(위상 정렬).
        /// </summary>
        private static List<Target> OrderForProcessing(TargetSet targets)
        {
            // 기본 순서: 경로 구성요소 수 내림차순(안정 정렬).
            List<Target> baseOrder = targets.Items.OrderByDescending(t => ComponentDepth(t.Path)).ToList();
            int count = baseOrder.Count;
            Dictionary<Target, int> indexOf = new Dictionary<Target, int>();
            for (int i = 0; i < count; i++)
            {
                indexOf[baseOrder[i]] = i;
            }

            // 간선 i → j: i를 j보다 먼저 처리합니다(j가 i 경로의 상위 구성요소).
            List<int>[] successors = new List<int>[count];
            int[] indegree = new int[count];
            for (int i = 0; i < count; i++)
            {
                successors[i] = new List<int>();
            }
            for (int i = 0; i < count; i++)
            {
                HashSet<int> dependsOn = new HashSet<int>();
                AddAncestorDependencies(targets, indexOf, System.IO.Path.GetDirectoryName(baseOrder[i].Path), i, dependsOn);
                foreach (int j in dependsOn)
                {
                    successors[i].Add(j);
                    indegree[j]++;
                }
            }

            // 먼저 처리할 수 있는 대상 가운데 기본 순서가 가장 앞선 것부터 꺼냅니다.
            SortedSet<int> ready = new SortedSet<int>();
            for (int i = 0; i < count; i++)
            {
                if (indegree[i] == 0)
                {
                    ready.Add(i);
                }
            }
            List<Target> ordered = new List<Target>(count);
            bool[] emitted = new bool[count];
            while (ready.Count > 0)
            {
                int i = ready.Min;
                ready.Remove(i);
                ordered.Add(baseOrder[i]);
                emitted[i] = true;
                foreach (int j in successors[i])
                {
                    if (--indegree[j] == 0)
                    {
                        ready.Add(j);
                    }
                }
            }
            // 링크 순환처럼 관계가 닫히는 비정상 구조가 있으면 남은 대상을 기본 순서로 덧붙입니다.
            for (int i = 0; i < count; i++)
            {
                if (!emitted[i])
                {
                    ordered.Add(baseOrder[i]);
                }
            }
            return ordered;
        }

        // start부터 루트까지 각 상위 구성요소가 다른 대상 항목(키 일치)이면 그 대상의 순번을 dependsOn에 넣습니다.
        private static void AddAncestorDependencies(TargetSet targets, Dictionary<Target, int> indexOf, string start, int self, HashSet<int> dependsOn)
        {
            string ancestor = start;
            while (!string.IsNullOrEmpty(ancestor))
            {
                Target dependency = targets.FindByKey(targets.KeyOfPath(ancestor));
                int j;
                if (dependency != null && indexOf.TryGetValue(dependency, out j) && j != self)
                {
                    dependsOn.Add(j);
                }
                ancestor = System.IO.Path.GetDirectoryName(ancestor);
            }
        }

        // 경로 구성요소 수(루트 제외)입니다. "D:\"는 0, "D:\a"는 1, "D:\a\b"는 2입니다.
        private static int ComponentDepth(string path)
        {
            string root = System.IO.Path.GetPathRoot(path) ?? string.Empty;
            return path.Substring(root.Length).Split(
                new[] { System.IO.Path.DirectorySeparatorChar }, StringSplitOptions.RemoveEmptyEntries).Length;
        }

        // 부모 폴더에서 name에 대응하는 항목의 저장 이름을 찾습니다. 검색은 폴더의 비교 규칙(대소문자 구분 여부)을 따릅니다.
        // 정확히 같은 이름이 있으면 그 이름, 대소문자만 다른 항목이 하나뿐이면 그 이름, 그 밖에는 null입니다.
        private static string FindStoredName(string parent, string name)
        {
            try
            {
                string onlyMatch = null;
                int matches = 0;
                foreach (string entry in Directory.EnumerateFileSystemEntries(parent, name))
                {
                    string entryName = System.IO.Path.GetFileName(entry);
                    if (string.Equals(entryName, name, StringComparison.Ordinal))
                    {
                        return entryName;
                    }
                    if (string.Equals(entryName, name, StringComparison.OrdinalIgnoreCase))
                    {
                        onlyMatch = entryName;
                        matches++;
                    }
                }
                return matches == 1 ? onlyMatch : null;
            }
            catch (Exception)
            {
                return null;
            }
        }

        /// <summary>
        /// 미리 보기에서 "예정"으로 판정한 새 이름을 폴더별로 예약해 둡니다.
        /// 실제 실행에서는 앞서 바꾼 이름이 디스크에 생기므로 디스크 검사로 충돌이 드러나지만,
        /// 미리 보기에서는 디스크가 바뀌지 않으므로 같은 새 이름으로 바뀔 예정인 항목끼리의 충돌을 이 예약으로 찾습니다.
        /// 폴더는 경로 문자열이 아니라 실제 항목 식별자(볼륨·파일 ID)로 구분합니다(D:\Work와 D:\work는 같은 폴더).
        /// 식별자를 얻을 수 없으면 대문자로 바꾼 경로로 구분합니다. 이름 비교는 폴더의 대소문자 구분 설정을 따릅니다.
        /// </summary>
        private sealed class PlannedNames
        {
            private readonly Dictionary<string, HashSet<string>> byFolder =
                new Dictionary<string, HashSet<string>>(StringComparer.Ordinal);

            public bool IsTaken(string folder, string name)
            {
                return GetSet(folder).Contains(name);
            }

            public void Reserve(string folder, string name)
            {
                GetSet(folder).Add(name);
            }

            private HashSet<string> GetSet(string folder)
            {
                // 정션·심볼릭 링크 경로로 접근한 폴더도 실제 폴더와 같은 예약을 쓰도록 링크를 따라간 식별자를 씁니다.
                string identity = NativeMethods.TryGetFileIdentity(folder, true);
                string key = identity != null ? "id:" + identity : "path:" + folder.ToUpperInvariant();
                HashSet<string> set;
                if (!byFolder.TryGetValue(key, out set))
                {
                    StringComparer comparer = NativeMethods.IsCaseSensitiveDirectory(folder)
                        ? StringComparer.Ordinal
                        : StringComparer.OrdinalIgnoreCase;
                    set = new HashSet<string>(comparer);
                    byFolder[key] = set;
                }
                return set;
            }
        }

        public static List<ItemResult> Run(IList<WorkItem> requests, bool dryRun)
        {
            List<ItemResult> results = new List<ItemResult>();
            TargetSet targets = new TargetSet();

            foreach (WorkItem request in requests)
            {
                string origin = request.Origin ?? string.Empty;
                string full;
                try
                {
                    full = NormalizePath(request.Path);
                }
                catch (Exception ex)
                {
                    results.Add(ItemResult.Failure(request.Path, "경로가 올바르지 않습니다: " + ex.Message, origin));
                    continue;
                }

                // 요청할 때 없던 경로는, 처리 시점에 같은 경로에 항목이 생겼어도 처리하지 않습니다(요청한 항목이 아님).
                if (!request.Existed)
                {
                    results.Add(ItemResult.Failure(full, "항목을 찾을 수 없습니다.", origin));
                    continue;
                }

                // 요청 시점의 원본과 같은 항목인지 확인합니다. 경로가 없거나 다른 항목으로 바뀌었으면
                // 식별자로 원본의 현재 경로를 찾습니다(다른 실행이 원본 또는 상위 폴더의 이름을 먼저 정리한 경우).
                bool exists = Directory.Exists(full) || File.Exists(full);
                bool selfHandled = false;
                if (!exists || !IsRequestedItem(full, request.Identity))
                {
                    string current = ResolveRequestedItem(full, request.Identity, request.Options);
                    if (current == null)
                    {
                        results.Add(ItemResult.Failure(full, exists
                            ? "요청한 뒤 이 경로의 항목이 다른 항목으로 바뀌었고, 요청한 항목을 찾지 못해 처리하지 않았습니다."
                            : "항목을 찾을 수 없습니다.", origin));
                        continue;
                    }
                    string currentName = System.IO.Path.GetFileName(current);
                    if (!string.IsNullOrEmpty(currentName) &&
                        string.Equals(NameNormalizer.Normalize(currentName, request.Options), currentName, StringComparison.Ordinal))
                    {
                        // 원본이 이미 정리된 이름으로 현재 경로에 있습니다. 이 항목 자체는 다시 처리하지 않습니다.
                        ItemResult already = new ItemResult();
                        already.OldPath = full;
                        already.NewPath = current;
                        already.Status = ItemStatus.Unchanged;
                        already.AlreadyNormalized = true;
                        already.Message = "요청한 뒤 이미 정리되어 현재 경로에 있습니다.";
                        already.Origins.Add(origin);
                        results.Add(already);
                        selfHandled = true;
                    }
                    full = current;
                }

                bool isDirectory = Directory.Exists(full);
                if (isDirectory && request.Depth != 0)
                {
                    if (IsReparsePoint(full))
                    {
                        // 링크 폴더를 직접 지정해도 링크가 가리키는 곳의 이름은 바꾸지 않습니다(범위 이탈 방지).
                        results.Add(ItemResult.Failure(full,
                            "링크(심볼릭 링크·정션) 폴더의 하위 항목은 처리하지 않습니다. 링크가 가리키는 폴더를 직접 지정하십시오.", origin));
                    }
                    else
                    {
                        CollectDescendants(full, request.Depth, request.Options, origin, targets, results);
                    }
                }
                if ((request.IncludeSelf || !isDirectory) && !selfHandled)
                {
                    targets.Add(full, null, request.Options, origin);
                }
            }

            // 하위 항목부터 처리해야 상위 폴더 이름이 바뀌어도 아직 처리하지 않은 하위 경로가 유효합니다(OrderForProcessing 참조).
            PlannedNames planned = dryRun ? new PlannedNames() : null;
            foreach (Target target in OrderForProcessing(targets))
            {
                ItemResult result = RenameOne(target.Path, target.Options, dryRun, planned);
                result.Origins.AddRange(target.Origins);
                results.Add(result);
            }
            return results;
        }

        // 경로의 항목이 요청 시점에 기록한 원본과 같은지 확인합니다.
        // 요청 시점에 항목은 있었지만 식별자를 얻지 못했으면(파일 ID를 제공하지 않는 파일 시스템 등) 비교할 수 없으므로
        // 같은 항목으로 봅니다. 이 경우 경로가 없어지면 원본을 찾을 수 없어 오류로 보고합니다.
        private static bool IsRequestedItem(string path, string identity)
        {
            if (string.IsNullOrEmpty(identity))
            {
                return true;
            }
            return string.Equals(NativeMethods.TryGetFileIdentity(path), identity, StringComparison.Ordinal);
        }

        // 절대 경로로 바꾸고 끝의 경로 구분자를 지웁니다(드라이브 루트는 그대로 둡니다).
        private static string NormalizePath(string path)
        {
            string full = System.IO.Path.GetFullPath(path);
            string root = System.IO.Path.GetPathRoot(full);
            if (full.Length > (root == null ? 0 : root.Length))
            {
                full = full.TrimEnd(System.IO.Path.DirectorySeparatorChar, System.IO.Path.AltDirectorySeparatorChar);
            }
            return full;
        }

        /// <summary>
        /// 요청 시점 식별자로 원본의 현재 경로를 찾습니다. 식별자가 없으면 찾지 않습니다(null).
        /// 1) 식별자로 직접 엽니다(OpenFileById). 이름이나 상위 폴더가 어떻게 바뀌었든 찾습니다(로컬 NTFS 등).
        /// 2) 1)을 쓸 수 없는 파일 시스템(SMB 공유 등)에서는, 요청 경로의 각 단계를 원래 이름 또는 정리된 이름으로 따라가며
        ///    마지막 항목의 식별자가 같은 경로를 찾습니다. namefix 자신이 이름을 정리한 경우를 복구합니다.
        /// 어느 쪽이든 식별자가 같은 항목만 인정하므로, 다른 항목을 원본으로 오인하지 않습니다.
        /// </summary>
        private static string ResolveRequestedItem(string path, string identity, NormalizeOptions options)
        {
            if (string.IsNullOrEmpty(identity))
            {
                return null;
            }
            string byId = NativeMethods.TryResolveCurrentPath(identity, path);
            if (byId != null)
            {
                return byId;
            }
            try
            {
                string root = System.IO.Path.GetPathRoot(path);
                if (string.IsNullOrEmpty(root))
                {
                    return null;
                }
                string[] names = path.Substring(root.Length).Split(
                    new[] { System.IO.Path.DirectorySeparatorChar }, StringSplitOptions.RemoveEmptyEntries);
                if (names.Length == 0)
                {
                    return null;
                }
                int budget = MaxResolveCandidates;
                return SearchByNormalizedNames(root, names, 0, identity, options, ref budget);
            }
            catch (Exception)
            {
                return null;
            }
        }

        // 경로 단계 index의 이름을 정리된 이름 → 원래 이름 순서로 시도하며 내려갑니다. 마지막 단계는 식별자가 같아야 합니다.
        private static string SearchByNormalizedNames(string current, string[] names, int index, string identity,
            NormalizeOptions options, ref int budget)
        {
            if (budget-- <= 0)
            {
                return null;
            }
            string original = System.IO.Path.Combine(current, names[index]);
            string normalizedName = NameNormalizer.Normalize(names[index], options);
            string normalized = string.Equals(normalizedName, names[index], StringComparison.Ordinal)
                ? null
                : System.IO.Path.Combine(current, normalizedName);
            bool last = index == names.Length - 1;
            foreach (string candidate in new[] { normalized, original })
            {
                if (candidate == null)
                {
                    continue;
                }
                if (last)
                {
                    // 하드 링크는 파일 ID가 같으므로, 링크가 여러 개인 파일은 식별자만으로 요청한 디렉터리 항목인지 정할 수 없어 인정하지 않습니다.
                    if ((File.Exists(candidate) || Directory.Exists(candidate)) &&
                        string.Equals(NativeMethods.TryGetFileIdentity(candidate), identity, StringComparison.Ordinal) &&
                        NativeMethods.GetHardLinkCount(candidate) == 1)
                    {
                        return candidate;
                    }
                }
                else if (Directory.Exists(candidate))
                {
                    string found = SearchByNormalizedNames(candidate, names, index + 1, identity, options, ref budget);
                    if (found != null)
                    {
                        return found;
                    }
                }
            }
            return null;
        }


        /// <summary>
        /// 폴더의 하위 항목을 수집합니다. depth가 1이면 직속 하위만, -1이면 모든 하위를 수집합니다.
        /// 깊은 트리에서 호출 스택이 넘치지 않도록 명시적 스택으로 순회합니다.
        /// 읽을 수 없는 폴더는 오류로 기록하고 나머지는 계속 수집합니다.
        /// </summary>
        private static void CollectDescendants(string directory, int depth, NormalizeOptions options, string origin,
            TargetSet targets, List<ItemResult> results)
        {
            Stack<KeyValuePair<string, int>> stack = new Stack<KeyValuePair<string, int>>();
            stack.Push(new KeyValuePair<string, int>(directory, 1));
            while (stack.Count > 0)
            {
                KeyValuePair<string, int> current = stack.Pop();
                string[] entries;
                try
                {
                    entries = Directory.GetFileSystemEntries(current.Key);
                }
                catch (Exception ex)
                {
                    results.Add(ItemResult.Failure(current.Key, "하위 항목을 읽지 못했습니다: " + ex.Message, origin));
                    continue;
                }

                foreach (string entry in entries)
                {
                    // 열거 결과의 이름은 디스크에 저장된 이름이므로 그대로 넘깁니다.
                    targets.Add(entry, System.IO.Path.GetFileName(entry), options, origin);
                    bool descend = depth < 0 || current.Value < depth;
                    if (descend && IsPlainDirectory(entry))
                    {
                        stack.Push(new KeyValuePair<string, int>(entry, current.Value + 1));
                    }
                }
            }
        }

        // 재분석 지점이 아닌 일반 폴더인지 확인합니다.
        private static bool IsPlainDirectory(string path)
        {
            try
            {
                FileAttributes attributes = File.GetAttributes(path);
                return (attributes & FileAttributes.Directory) != 0 && (attributes & FileAttributes.ReparsePoint) == 0;
            }
            catch (Exception)
            {
                return false;
            }
        }

        // 재분석 지점(심볼릭 링크·정션 등)인지 확인합니다. 확인할 수 없으면 안전하게 링크로 취급합니다.
        private static bool IsReparsePoint(string path)
        {
            try
            {
                return (File.GetAttributes(path) & FileAttributes.ReparsePoint) != 0;
            }
            catch (Exception)
            {
                return true;
            }
        }

        /// <summary>항목 한 개의 이름을 정리합니다. planned는 미리 보기에서만 사용합니다.</summary>
        private static ItemResult RenameOne(string path, NormalizeOptions options, bool dryRun, PlannedNames planned)
        {
            ItemResult result = new ItemResult();
            result.OldPath = path;
            result.NewPath = path;
            try
            {
                string name = System.IO.Path.GetFileName(path);
                string parent = System.IO.Path.GetDirectoryName(path);
                if (string.IsNullOrEmpty(name) || string.IsNullOrEmpty(parent))
                {
                    return Fail(result, "드라이브 루트는 이름을 바꿀 수 없습니다.");
                }

                bool isDirectory = Directory.Exists(path);
                if (!isDirectory && !File.Exists(path))
                {
                    return Fail(result, "항목을 찾을 수 없습니다.");
                }

                string newName = NameNormalizer.Normalize(name, options);
                if (string.Equals(newName, name, StringComparison.Ordinal))
                {
                    result.Status = ItemStatus.Unchanged;
                    return result;
                }

                string newPath = System.IO.Path.Combine(parent, newName);
                result.NewPath = newPath;

                bool taken = TargetIsTaken(parent, name, newName, newPath) ||
                             (planned != null && planned.IsTaken(parent, newName));
                if (taken)
                {
                    result.Status = ItemStatus.Conflict;
                    result.Message = "같은 이름의 항목이 이미 있습니다.";
                    return result;
                }

                if (dryRun)
                {
                    planned.Reserve(parent, newName);
                    result.Status = ItemStatus.Planned;
                    return result;
                }

                if (isDirectory)
                {
                    Directory.Move(path, newPath);
                }
                else
                {
                    File.Move(path, newPath);
                }
                result.Status = ItemStatus.Renamed;
                NativeMethods.NotifyRename(path, newPath, isDirectory);
                return result;
            }
            catch (Exception ex)
            {
                return Fail(result, ex.Message);
            }
        }

        /// <summary>
        /// 새 이름을 원본이 아닌 다른 디렉터리 항목이 쓰고 있는지 판정합니다.
        /// 부모 폴더를 새 이름으로 검색하면 파일 시스템의 비교 규칙(대소문자·정규화 무시 여부, 짧은 이름)에 따라
        /// 새 이름에 대응하는 실제 항목이 돌아옵니다. 돌아온 항목의 저장 이름이 모두 원본 자신의 이름일 때만
        /// "같은 항목을 다른 표기로 연 것"(예: 정규화를 구분하지 않는 macOS SMB 공유)으로 보고 이름 변경을 허용합니다.
        /// 그 밖의 항목이 하나라도 있으면(하드 링크, 대소문자만 다른 항목 포함) 충돌입니다. 판정할 수 없어도 충돌로 봅니다.
        /// </summary>
        private static bool TargetIsTaken(string parent, string name, string newName, string newPath)
        {
            if (!File.Exists(newPath) && !Directory.Exists(newPath))
            {
                return false;
            }
            try
            {
                bool any = false;
                foreach (string entry in Directory.EnumerateFileSystemEntries(parent, newName))
                {
                    any = true;
                    if (!string.Equals(System.IO.Path.GetFileName(entry), name, StringComparison.Ordinal))
                    {
                        return true;
                    }
                }
                // 새 이름으로 열리는데 검색 결과가 없으면 판정할 수 없으므로 충돌로 봅니다.
                return !any;
            }
            catch (Exception)
            {
                return true;
            }
        }

        private static ItemResult Fail(ItemResult result, string message)
        {
            result.Status = ItemStatus.Error;
            result.Message = message;
            return result;
        }
    }
}
