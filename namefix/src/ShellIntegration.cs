// namefix - 탐색기 오른쪽 클릭 메뉴 등록
//
// 현재 사용자 영역(HKCU\Software\Classes)에만 등록하므로 관리자 권한이 필요 없습니다.
// 등록하는 메뉴:
//   *\shell\namefix                       파일 선택      → 선택한 파일 이름 정리
//   Directory\shell\namefix               폴더 선택      → 선택한 폴더 자신의 이름 정리
//   Directory\shell\namefix.recursive     폴더 선택      → 폴더와 모든 하위 항목 이름 정리
//   Directory\Background\shell\namefix    폴더 빈 공간   → 현재 폴더 안 항목 이름 정리
//
// Windows 11의 새 오른쪽 클릭 메뉴에서는 "추가 옵션 표시"(Shift+F10) 아래에 나타납니다.
// 경로 인수("%1", "%V")는 항상 명령줄 끝에 둡니다. 드라이브 루트(D:\)가 "D:\"로 인용되면
// \" 가 따옴표 이스케이프로 해석되는데, 끝에 두어야 다른 인수가 깨지지 않습니다(CommandLine.CleanPathArgument 참조).
using System;
using System.Collections.Generic;
using Microsoft.Win32;

namespace NameFix
{
    internal static class ShellIntegration
    {
        private const string DefaultClassesRoot = @"Software\Classes\";

        // 테스트에서 실제 메뉴 등록을 건드리지 않도록 등록 위치를 바꾸는 환경 변수입니다.
        // 잘못 쓰여 다른 레지스트리를 지우지 않도록 HKCU\Software\namefix-test 로 시작하는 경로만 허용합니다.
        // 값이 있는데 허용 범위가 아니면 실제 메뉴(Software\Classes)로 되돌아가지 않고 작업을 실패시킵니다.
        private const string TestRootVariable = "NAMEFIX_TEST_REGISTRY_ROOT";
        private const string TestRootPrefix = @"Software\namefix-test";

        private static string ClassesRoot
        {
            get
            {
                string testRoot = Environment.GetEnvironmentVariable(TestRootVariable);
                if (string.IsNullOrEmpty(testRoot))
                {
                    return DefaultClassesRoot;
                }
                if (!testRoot.StartsWith(TestRootPrefix, StringComparison.OrdinalIgnoreCase) ||
                    testRoot.IndexOf("..", StringComparison.Ordinal) >= 0)
                {
                    throw new InvalidOperationException(
                        TestRootVariable + " 값이 허용 범위(HKCU\\" + TestRootPrefix + "...)가 아닙니다: " + testRoot);
                }
                return testRoot.TrimEnd('\\') + "\\";
            }
        }

        private sealed class VerbSpec
        {
            public readonly string KeyPath;
            public readonly string Label;
            public readonly string Arguments;

            public VerbSpec(string keyPath, string label, string arguments)
            {
                KeyPath = keyPath;
                Label = label;
                Arguments = arguments;
            }
        }

        private static readonly VerbSpec[] Verbs =
        {
            new VerbSpec(@"*\shell\namefix", "파일명 정리 (자소 분리·URL 인코딩)", "--gui \"%1\""),
            new VerbSpec(@"Directory\shell\namefix", "폴더 이름 정리 (자소 분리·URL 인코딩)", "--gui \"%1\""),
            new VerbSpec(@"Directory\shell\namefix.recursive", "폴더와 하위 항목 이름 모두 정리", "--gui --recursive \"%1\""),
            new VerbSpec(@"Directory\Background\shell\namefix", "이 폴더의 파일명 정리 (자소 분리·URL 인코딩)", "--gui --children \"%V\"")
        };

        /// <summary>등록할(또는 등록된) 메뉴 키 경로 목록입니다(HKCU 기준).</summary>
        public static IEnumerable<string> KeyPaths
        {
            get
            {
                string root = ClassesRoot;
                foreach (VerbSpec verb in Verbs)
                {
                    yield return root + verb.KeyPath;
                }
            }
        }

        /// <summary>탐색기 메뉴를 등록합니다. 이미 있으면 현재 실행 파일 경로로 덮어씁니다.</summary>
        /// <param name="guiExecutable">메뉴에서 실행할 namefixw.exe의 전체 경로</param>
        public static void Register(string guiExecutable)
        {
            string root = ClassesRoot;
            foreach (VerbSpec verb in Verbs)
            {
                using (RegistryKey key = Registry.CurrentUser.CreateSubKey(root + verb.KeyPath))
                {
                    key.SetValue("MUIVerb", verb.Label, RegistryValueKind.String);
                    // Player: 선택 항목 수와 관계없이 메뉴를 표시합니다(기본값은 15개를 넘으면 메뉴를 숨깁니다).
                    key.SetValue("MultiSelectModel", "Player", RegistryValueKind.String);
                    using (RegistryKey command = key.CreateSubKey("command"))
                    {
                        command.SetValue(string.Empty, "\"" + guiExecutable + "\" " + verb.Arguments, RegistryValueKind.String);
                    }
                }
            }
        }

        /// <summary>
        /// 탐색기 메뉴 등록을 해제합니다. 등록되어 있지 않은 키는 오류로 보지 않습니다.
        /// 삭제 권한이 없으면 예외가 그대로 올라갑니다. 삭제 뒤에도 키가 남아 있으면 예외를 던집니다.
        /// </summary>
        public static void Unregister()
        {
            List<string> keyPaths = new List<string>(KeyPaths);
            foreach (string keyPath in keyPaths)
            {
                Registry.CurrentUser.DeleteSubKeyTree(keyPath, false);
            }

            List<string> remaining = new List<string>();
            foreach (string keyPath in keyPaths)
            {
                using (RegistryKey key = Registry.CurrentUser.OpenSubKey(keyPath))
                {
                    if (key != null)
                    {
                        remaining.Add(keyPath);
                    }
                }
            }
            if (remaining.Count > 0)
            {
                throw new InvalidOperationException("다음 메뉴 키가 남아 있습니다: HKCU\\" + string.Join(", HKCU\\", remaining.ToArray()));
            }
        }
    }
}
