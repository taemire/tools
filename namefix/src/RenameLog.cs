// namefix - 변경 기록
//
// 실제로 이름을 바꾼 항목, 충돌·오류 항목과 실행 요약을 UTF-8 탭 구분 텍스트로 남깁니다.
// 잘못 바꾼 이름을 되돌릴 때 OLD/NEW 경로를 확인하는 용도입니다.
// 형식(한 줄 한 항목):
//   <시각>\tRENAMED|CONFLICT|ERROR\t<이전 경로>\t<새 경로>\t<메시지>
//   <시각>\tSUMMARY\tsource=<cli|gui>\tpid=<n>\tinputs=<n>\trenamed=<n>\tconflict=<n>\terror=<n>\tunchanged=<n>
using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Globalization;
using System.IO;
using System.Security.Cryptography;
using System.Text;
using System.Threading;

namespace NameFix
{
    internal static class RenameLog
    {
        // 로그 파일이 이 크기를 넘으면 namefix.log.1로 옮기고 새로 씁니다.
        private const long MaxLogBytes = 1024 * 1024;

        /// <summary>
        /// 로그 경로입니다. 환경 변수 NAMEFIX_LOG가 있으면 그 경로를 사용합니다(테스트용).
        /// 기본값은 %LOCALAPPDATA%\namefix\namefix.log입니다.
        /// </summary>
        public static string LogPath
        {
            get
            {
                string overridePath = Environment.GetEnvironmentVariable("NAMEFIX_LOG");
                if (!string.IsNullOrEmpty(overridePath))
                {
                    return overridePath;
                }
                return Path.Combine(
                    Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "namefix", "namefix.log");
            }
        }

        /// <summary>
        /// 처리 결과를 기록합니다. 미리 보기(--dry-run)는 실제 변경이 없으므로 기록하지 않습니다.
        /// 기록에 실패해도 이름 정리 결과에는 영향을 주지 않습니다.
        /// </summary>
        public static void Write(IList<ItemResult> results, Summary summary, string source, int inputs, bool dryRun)
        {
            if (dryRun)
            {
                return;
            }
            try
            {
                string path = Path.GetFullPath(LogPath);
                string directory = Path.GetDirectoryName(path);
                if (!string.IsNullOrEmpty(directory))
                {
                    Directory.CreateDirectory(directory);
                }

                string stamp = DateTimeOffset.Now.ToString("yyyy-MM-dd'T'HH:mm:ss.fffzzz", CultureInfo.InvariantCulture);
                StringBuilder sb = new StringBuilder();
                foreach (ItemResult r in results)
                {
                    string kind;
                    switch (r.Status)
                    {
                        case ItemStatus.Renamed: kind = "RENAMED"; break;
                        case ItemStatus.Conflict: kind = "CONFLICT"; break;
                        case ItemStatus.Error: kind = "ERROR"; break;
                        default: continue;
                    }
                    sb.Append(stamp).Append('\t').Append(kind)
                      .Append('\t').Append(Clean(r.OldPath))
                      .Append('\t').Append(Clean(r.NewPath))
                      .Append('\t').Append(Clean(r.Message))
                      .Append("\r\n");
                }
                sb.Append(stamp).Append("\tSUMMARY")
                  .Append("\tsource=").Append(source)
                  .Append("\tpid=").Append(Process.GetCurrentProcess().Id.ToString(CultureInfo.InvariantCulture))
                  .Append("\tinputs=").Append(inputs.ToString(CultureInfo.InvariantCulture))
                  .Append("\trenamed=").Append(summary.Renamed.ToString(CultureInfo.InvariantCulture))
                  .Append("\tconflict=").Append(summary.Conflicts.ToString(CultureInfo.InvariantCulture))
                  .Append("\terror=").Append(summary.Errors.ToString(CultureInfo.InvariantCulture))
                  .Append("\tunchanged=").Append(summary.Unchanged.ToString(CultureInfo.InvariantCulture))
                  .Append("\talready=").Append(summary.AlreadyNormalized.ToString(CultureInfo.InvariantCulture))
                  .Append("\r\n");

                byte[] bytes = new UTF8Encoding(false).GetBytes(sb.ToString());

                // 여러 프로세스가 동시에 교체 조건을 만나면 서로의 .1 파일을 덮어쓸 수 있으므로,
                // 크기 확인·교체·추가 기록 전체를 로그 경로별 프로세스 간 뮤텍스로 직렬화합니다.
                // 같은 사용자의 다른 로그온 세션(원격 데스크톱 등)도 같은 로그를 쓰므로 세션을 넘는 Global\ 이름을 씁니다.
                using (Mutex mutex = OpenLogMutex(path))
                {
                    bool owned = Acquire(mutex);
                    try
                    {
                        if (owned)
                        {
                            RotateIfLarge(path);
                        }
                        AppendWithRetry(path, bytes);
                    }
                    finally
                    {
                        if (owned)
                        {
                            mutex.ReleaseMutex();
                        }
                    }
                }
            }
            catch (Exception)
            {
                // 기록 실패는 무시합니다.
            }
        }

        // 로그 경로별 뮤텍스를 엽니다. 뮤텍스 이름에는 '\'를 쓸 수 없으므로 경로의 SHA-1 값을 씁니다.
        // Global\ 이름은 모든 세션이 공유합니다. 로그 경로는 사용자별(%LOCALAPPDATA%)이므로 다른 사용자와 이름이 겹치지 않습니다.
        // 만들거나 열 수 없으면 null을 돌려주며, 이때는 교체 없이 추가 기록만 합니다.
        private static Mutex OpenLogMutex(string path)
        {
            try
            {
                using (SHA1 sha = SHA1.Create())
                {
                    byte[] hash = sha.ComputeHash(Encoding.UTF8.GetBytes(path.ToUpperInvariant()));
                    StringBuilder sb = new StringBuilder("Global\\namefix-log-", 60);
                    foreach (byte b in hash)
                    {
                        sb.Append(b.ToString("x2", CultureInfo.InvariantCulture));
                    }
                    return new Mutex(false, sb.ToString());
                }
            }
            catch (Exception)
            {
                return null;
            }
        }

        // 최대 5초 기다립니다. 얻지 못하면 교체 없이 추가 기록만 합니다(교체는 다음 실행에서 수행).
        private static bool Acquire(Mutex mutex)
        {
            if (mutex == null)
            {
                return false;
            }
            try
            {
                return mutex.WaitOne(5000);
            }
            catch (AbandonedMutexException)
            {
                return true;
            }
        }

        // 다른 프로그램이 로그를 잠깐 열고 있을 수 있으므로 공유 위반이면 짧게 기다렸다가 다시 시도합니다.
        private static void AppendWithRetry(string path, byte[] bytes)
        {
            for (int attempt = 0; attempt < 20; attempt++)
            {
                try
                {
                    using (FileStream stream = new FileStream(path, FileMode.Append, FileAccess.Write, FileShare.Read))
                    {
                        stream.Write(bytes, 0, bytes.Length);
                    }
                    return;
                }
                catch (IOException)
                {
                    Thread.Sleep(50);
                }
            }
        }

        // 탭·줄바꿈이 섞이면 한 줄 한 항목 형식이 깨지므로 공백으로 바꿉니다.
        private static string Clean(string text)
        {
            if (string.IsNullOrEmpty(text))
            {
                return string.Empty;
            }
            return text.Replace('\t', ' ').Replace('\r', ' ').Replace('\n', ' ');
        }

        private static void RotateIfLarge(string path)
        {
            try
            {
                FileInfo info = new FileInfo(path);
                if (!info.Exists || info.Length <= MaxLogBytes)
                {
                    return;
                }
                // 기존 .1을 먼저 지우면 이동이 실패했을 때 기록을 잃으므로, .1이 있으면 ReplaceFile로 한 번에 바꿉니다.
                string previous = path + ".1";
                if (File.Exists(previous))
                {
                    File.Replace(path, previous, null, true);
                }
                else
                {
                    File.Move(path, previous);
                }
            }
            catch (Exception)
            {
                // 다른 프로그램이 로그를 열고 있으면 이번에는 교체하지 않습니다. 기존 기록은 그대로 남습니다.
            }
        }
    }
}
