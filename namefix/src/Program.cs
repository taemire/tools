// namefix - 파일·폴더 이름의 자소 분리(NFD)와 URL 인코딩을 정리하는 도구
//
// 같은 소스에서 실행 파일 두 개를 만듭니다(build.bat 참조).
//   namefix.exe  : 콘솔 하위 시스템. 터미널·스크립트·테스트에서 사용합니다.
//   namefixw.exe : Windows 하위 시스템(WINEXE 정의). 탐색기 오른쪽 클릭 메뉴에서 콘솔 창 없이 실행되며,
//                  항상 대화상자 모드(--gui)로 동작합니다.
//
// 종료 코드: 0 정상(바꿀 것이 없는 경우 포함), 1 충돌·오류 있음, 2 사용법 오류
using System;
using System.Collections.Generic;
using System.IO;
using System.Reflection;
using System.Text;
using System.Windows.Forms;

namespace NameFix
{
    internal static class Program
    {
        internal const string Version = "0.1.0";

        private const int ExitOk = 0;
        private const int ExitProblem = 1;
        private const int ExitUsage = 2;

        // 결과 대화상자에 항목별로 보여 줄 최대 줄 수입니다.
        private const int MaxDialogLines = 12;

        private const string DialogTitle = "파일명 정리 - namefix";

        private const string HelpText =
            "namefix " + Version + " - 파일·폴더 이름의 자소 분리(NFD)와 URL 인코딩을 정리합니다.\r\n" +
            "\r\n" +
            "사용법: namefix [옵션] <경로>...\r\n" +
            "\r\n" +
            "  -n, --dry-run     이름을 바꾸지 않고 바뀔 이름만 보여 줍니다.\r\n" +
            "  -r, --recursive   폴더와 모든 하위 항목의 이름을 정리합니다.\r\n" +
            "  -c, --children    폴더 자신은 두고 안의 항목만 정리합니다(-r과 함께 쓰면 모든 하위 항목).\r\n" +
            "      --no-url      URL 인코딩(%XX)을 풀지 않습니다.\r\n" +
            "      --no-nfc      자소 분리(NFD)를 합치지 않습니다.\r\n" +
            "      --gui         결과를 대화상자로 보여 주고, 동시에 실행된 요청을 한 번에 처리합니다.\r\n" +
            "  -q, --quiet       항목별 결과와 합계를 출력하지 않습니다(충돌·오류는 출력).\r\n" +
            "  -v, --verbose     바뀌지 않은 항목도 출력합니다.\r\n" +
            "      --register    탐색기 오른쪽 클릭 메뉴를 현재 사용자에게 등록합니다.\r\n" +
            "      --unregister  탐색기 오른쪽 클릭 메뉴 등록을 해제합니다.\r\n" +
            "  -h, --help        도움말을 출력합니다.\r\n" +
            "  -V, --version     버전을 출력합니다.\r\n" +
            "\r\n" +
            "종료 코드: 0 정상, 1 충돌·오류 있음, 2 사용법 오류\r\n" +
            "변경 기록: %LOCALAPPDATA%\\namefix\\namefix.log";

        [STAThread]
        private static int Main(string[] args)
        {
            CommandLine cmd = new CommandLine();
            string error;
            bool parsed = cmd.Parse(args, out error);
#if WINEXE
            // 탐색기용 실행 파일은 콘솔이 없으므로 항상 대화상자 모드로 동작합니다.
            cmd.Gui = true;
#endif
            if (!parsed)
            {
                return Fail(cmd.Gui, error + "\r\n도움말: namefix --help", ExitUsage);
            }
            if (cmd.ShowHelp)
            {
                Inform(cmd.Gui, HelpText);
                return ExitOk;
            }
            if (cmd.ShowVersion)
            {
                Inform(cmd.Gui, "namefix " + Version);
                return ExitOk;
            }
            if (cmd.Register || cmd.Unregister)
            {
                return RunRegistration(cmd);
            }
            if (cmd.Paths.Count == 0)
            {
                return Fail(cmd.Gui, "정리할 경로를 지정하십시오.\r\n도움말: namefix --help", ExitUsage);
            }

            try
            {
                return Run(cmd);
            }
            catch (Exception ex)
            {
                // 처리 중 예기치 않은 오류는 콘솔 또는 대화상자로 알리고 종료 코드 1로 끝냅니다.
                return Fail(cmd.Gui, "예기치 않은 오류로 작업을 끝내지 못했습니다: " + ex.Message, ExitProblem);
            }
        }

        private static int Run(CommandLine cmd)
        {
            // 절대 경로를 확정하지 못한 경로는 다른 프로세스에 보내지 않고 이 프로세스에서 오류로 보고합니다.
            List<ItemResult> unresolved = new List<ItemResult>();
            List<WorkItem> requests = new List<WorkItem>();
            foreach (string path in cmd.Paths)
            {
                // 다른 프로세스(수집자)는 작업 폴더가 다르므로, 보내기 전에 이 프로세스의 작업 폴더 기준 절대 경로로 확정합니다.
                string fullPath;
                string error;
                if (!TryGetAbsolutePath(path, out fullPath, out error))
                {
                    unresolved.Add(ItemResult.Failure(path, "경로를 해석할 수 없습니다: " + error, string.Empty));
                    continue;
                }
                WorkItem item = new WorkItem();
                item.Path = fullPath;
                // 요청 시점의 존재 여부와 원본 식별자를 기록합니다. 처리 시점에 경로가 바뀌었으면 이 식별자로 원본을 찾고,
                // 경로에 다른 항목이 있거나 요청할 때 없던 경로이면 처리하지 않습니다.
                item.Existed = File.Exists(fullPath) || Directory.Exists(fullPath);
                item.Identity = item.Existed ? (NativeMethods.TryGetFileIdentity(fullPath) ?? string.Empty) : string.Empty;
                item.IncludeSelf = !cmd.Children;
                item.Depth = cmd.Recursive ? -1 : (cmd.Children ? 1 : 0);
                // 다중 실행 집계 시 다른 프로세스의 요청과 섞이므로 변환 옵션을 요청마다 복사해 둡니다.
                item.Options.UrlDecode = cmd.Normalize.UrlDecode;
                item.Options.Nfc = cmd.Normalize.Nfc;
                requests.Add(item);
            }

            Collector.TestPause();

            // 탐색기에서 여러 항목을 선택하면 항목마다 프로세스가 실행되므로,
            // 대화상자 모드에서는 동시에 실행된 요청을 한 프로세스로 모아 결과를 한 번만 보여 줍니다.
            CollectorBatch batch = null;
            if (cmd.Gui && !cmd.DryRun && unresolved.Count == 0)
            {
                int forwardedExitCode;
                batch = Collector.Gather(requests, out forwardedExitCode);
                if (batch == null)
                {
                    // 다른 수집자가 이 요청의 처리를 마쳤습니다. 결과는 수집자가 대화상자로 알렸으며, 종료 코드만 돌려줍니다.
                    return forwardedExitCode;
                }
                requests = batch.Items;
            }
            // 이 프로세스 자신의 요청 출처입니다. 종료 코드는 자기 요청의 결과로 정합니다.
            string ownOrigin = batch != null ? batch.OwnId : string.Empty;

            List<ItemResult> results;
            Summary summary;
            try
            {
                Collector.TestCrashPoint("before-process", batch);
                // 실제 이름 변경은 사용자 단위로 직렬화합니다. 미리 보기는 디스크를 바꾸지 않으므로 잠그지 않습니다.
                if (cmd.DryRun)
                {
                    results = Renamer.Run(requests, true);
                }
                else
                {
                    using (ExecutionLock.Acquire())
                    {
                        results = Renamer.Run(requests, false);
                    }
                }
                Collector.TestCrashPoint("after-process", batch);

                results.InsertRange(0, unresolved);
                summary = Summary.From(results);
                RenameLog.Write(results, summary, cmd.Gui ? "gui" : "cli", requests.Count + unresolved.Count, cmd.DryRun);
                if (batch != null)
                {
                    List<ItemResult> finished = results;
                    batch.SignalCompleted(delegate(string id) { return HasProblem(finished, id); });
                }
            }
            finally
            {
                // 수명 뮤텍스를 놓아 기다리던 프로세스를 깨웁니다. 완료를 알리지 못했으면(예외) 그쪽이 직접 다시 처리합니다.
                if (batch != null)
                {
                    batch.Dispose();
                }
            }

            if (cmd.Gui)
            {
                ShowResultDialog(results, summary, cmd.DryRun);
            }
            else
            {
                PrintResults(results, summary, cmd);
            }
            // 수집자는 다른 요청의 결과도 대화상자로 보여 주지만, 종료 코드는 자기 요청의 결과로 정합니다.
            return HasProblem(results, ownOrigin) ? ExitProblem : ExitOk;
        }

        // 출처가 origin인 결과 가운데 충돌·오류가 있는지 확인합니다.
        private static bool HasProblem(List<ItemResult> results, string origin)
        {
            foreach (ItemResult r in results)
            {
                if (r.IsProblem && r.Origins.Contains(origin))
                {
                    return true;
                }
            }
            return false;
        }

        // 상대 경로를 현재 작업 폴더 기준 절대 경로로 바꿉니다. 해석할 수 없으면(잘못된 문자, 경로 길이 초과 등) false입니다.
        private static bool TryGetAbsolutePath(string path, out string fullPath, out string error)
        {
            try
            {
                fullPath = Path.GetFullPath(path);
                error = null;
                return true;
            }
            catch (Exception ex)
            {
                fullPath = null;
                error = ex.Message;
                return false;
            }
        }

        // 콘솔 출력: 변경·예정은 표준 출력, 충돌·오류는 표준 오류로 보냅니다.
        private static void PrintResults(List<ItemResult> results, Summary summary, CommandLine cmd)
        {
            foreach (ItemResult r in results)
            {
                switch (r.Status)
                {
                    case ItemStatus.Renamed:
                        if (!cmd.Quiet)
                        {
                            Console.Out.WriteLine("변경  " + r.OldPath + " -> " + NameOf(r.NewPath));
                        }
                        break;
                    case ItemStatus.Planned:
                        if (!cmd.Quiet)
                        {
                            Console.Out.WriteLine("예정  " + r.OldPath + " -> " + NameOf(r.NewPath));
                        }
                        break;
                    case ItemStatus.Unchanged:
                        if (r.AlreadyNormalized && !cmd.Quiet)
                        {
                            // 요청한 경로가 이미 정리된 이름으로 바뀌어 있는 경우입니다(같은 요청의 재실행 등).
                            Console.Out.WriteLine("유지  " + r.OldPath + " -> " + NameOf(r.NewPath) + ": " + r.Message);
                        }
                        else if (cmd.Verbose && !cmd.Quiet)
                        {
                            Console.Out.WriteLine("유지  " + r.OldPath);
                        }
                        break;
                    case ItemStatus.Conflict:
                        Console.Error.WriteLine("충돌  " + r.OldPath + " -> " + NameOf(r.NewPath) + ": " + r.Message);
                        break;
                    default:
                        Console.Error.WriteLine("오류  " + r.OldPath + ": " + r.Message);
                        break;
                }
            }
            if (!cmd.Quiet)
            {
                Console.Out.WriteLine(string.Format(
                    "합계: 변경 {0}, 예정 {1}, 충돌 {2}, 오류 {3}, 유지 {4}",
                    summary.Renamed, summary.Planned, summary.Conflicts, summary.Errors, summary.Unchanged));
            }
        }

        // 대화상자 출력: 문제 없이 이름을 바꿨으면 탐색기에 바뀐 이름이 바로 보이므로 대화상자를 띄우지 않습니다.
        // 요청한 항목이 모두 이미 정리된 이름으로 바뀌어 있는 경우(같은 요청의 중복 처리)도 같은 성공으로 보고 띄우지 않습니다.
        // 충돌·오류가 있거나, 바꿀 것이 없었거나, 미리 보기일 때만 결과를 알립니다.
        private static void ShowResultDialog(List<ItemResult> results, Summary summary, bool dryRun)
        {
            bool problems = summary.Conflicts + summary.Errors > 0;
            bool onlyAlreadyNormalized = summary.AlreadyNormalized > 0 && summary.AlreadyNormalized == summary.Unchanged;
            if (!problems && !dryRun && (summary.Renamed > 0 || onlyAlreadyNormalized))
            {
                return;
            }

            StringBuilder text = new StringBuilder();
            if (!problems && summary.Renamed == 0 && summary.Planned == 0)
            {
                text.Append("정리할 이름이 없습니다.\r\n선택한 항목은 이미 정상 이름입니다.");
                ShowMessage(text.ToString(), MessageBoxIcon.Information);
                return;
            }

            if (dryRun)
            {
                text.AppendFormat("바뀔 예정 {0}건, 충돌 {1}건, 오류 {2}건, 유지 {3}건\r\n\r\n",
                    summary.Planned, summary.Conflicts, summary.Errors, summary.Unchanged);
            }
            else
            {
                text.AppendFormat("변경 {0}건, 충돌 {1}건, 오류 {2}건, 유지 {3}건\r\n\r\n",
                    summary.Renamed, summary.Conflicts, summary.Errors, summary.Unchanged);
            }

            List<string> lines = new List<string>();
            foreach (ItemResult r in results)
            {
                switch (r.Status)
                {
                    case ItemStatus.Conflict:
                        lines.Add("[충돌] " + NameOf(r.OldPath) + " → " + NameOf(r.NewPath) + " : " + r.Message);
                        break;
                    case ItemStatus.Error:
                        lines.Add("[오류] " + NameOf(r.OldPath) + " : " + r.Message);
                        break;
                    case ItemStatus.Planned:
                        lines.Add("[예정] " + NameOf(r.OldPath) + " → " + NameOf(r.NewPath));
                        break;
                }
            }
            for (int i = 0; i < lines.Count && i < MaxDialogLines; i++)
            {
                text.Append(lines[i]).Append("\r\n");
            }
            if (lines.Count > MaxDialogLines)
            {
                text.AppendFormat("… 외 {0}건\r\n", lines.Count - MaxDialogLines);
            }
            if (!dryRun)
            {
                text.Append("\r\n변경 기록: ").Append(RenameLog.LogPath);
            }
            ShowMessage(text.ToString(), problems ? MessageBoxIcon.Warning : MessageBoxIcon.Information);
        }

        private static int RunRegistration(CommandLine cmd)
        {
            try
            {
                if (cmd.Unregister)
                {
                    ShellIntegration.Unregister();
                    Inform(cmd.Gui, "탐색기 메뉴 등록을 해제했습니다.");
                    return ExitOk;
                }
                // 메뉴에서는 콘솔 창이 뜨지 않는 namefixw.exe를 실행합니다. 같은 폴더에 있어야 합니다.
                string directory = Path.GetDirectoryName(Assembly.GetExecutingAssembly().Location);
                string guiExecutable = Path.Combine(directory, "namefixw.exe");
                if (!File.Exists(guiExecutable))
                {
                    return Fail(cmd.Gui, "namefixw.exe를 찾을 수 없습니다: " + guiExecutable, ExitProblem);
                }
                ShellIntegration.Register(guiExecutable);
                Inform(cmd.Gui, "탐색기 메뉴를 등록했습니다: " + guiExecutable);
                return ExitOk;
            }
            catch (Exception ex)
            {
                return Fail(cmd.Gui, "탐색기 메뉴 등록 작업에 실패했습니다: " + ex.Message, ExitProblem);
            }
        }

        private static string NameOf(string path)
        {
            if (string.IsNullOrEmpty(path))
            {
                return string.Empty;
            }
            try
            {
                string name = Path.GetFileName(path);
                return string.IsNullOrEmpty(name) ? path : name;
            }
            catch (ArgumentException)
            {
                return path;
            }
        }

        private static int Fail(bool gui, string message, int exitCode)
        {
            if (gui)
            {
                ShowMessage(message, MessageBoxIcon.Error);
            }
            else
            {
                Console.Error.WriteLine(message);
            }
            return exitCode;
        }

        private static void Inform(bool gui, string message)
        {
            if (gui)
            {
                ShowMessage(message, MessageBoxIcon.Information);
            }
            else
            {
                Console.Out.WriteLine(message);
            }
        }

        // 환경 변수 NAMEFIX_NO_DIALOG=1이면 대화상자를 띄우지 않습니다(자동 테스트용).
        private static void ShowMessage(string text, MessageBoxIcon icon)
        {
            if (Environment.GetEnvironmentVariable("NAMEFIX_NO_DIALOG") == "1")
            {
                return;
            }
            NativeMethods.TrySetDpiAware();
            Application.EnableVisualStyles();
            // 탐색기 창 뒤에 가려지지 않도록 항상 위에 있는 보이지 않는 소유자 창을 둡니다.
            using (Form owner = new Form())
            {
                owner.TopMost = true;
                owner.ShowInTaskbar = false;
                MessageBox.Show(owner, text, DialogTitle, MessageBoxButtons.OK, icon);
            }
        }
    }

    /// <summary>GNU 형식 명령줄 옵션을 해석합니다(짧은 옵션 묶음 -nr, 긴 옵션 --dry-run, 옵션 끝 표시 --).</summary>
    internal sealed class CommandLine
    {
        public bool DryRun;
        public bool Recursive;
        public bool Children;
        public bool Gui;
        public bool Quiet;
        public bool Verbose;
        public bool ShowHelp;
        public bool ShowVersion;
        public bool Register;
        public bool Unregister;
        public readonly NormalizeOptions Normalize = new NormalizeOptions();
        public readonly List<string> Paths = new List<string>();

        public bool Parse(string[] args, out string error)
        {
            error = null;
            bool endOfOptions = false;
            foreach (string arg in args)
            {
                if (!endOfOptions && arg == "--")
                {
                    endOfOptions = true;
                    continue;
                }
                if (!endOfOptions && arg.StartsWith("--", StringComparison.Ordinal))
                {
                    if (!ApplyLongOption(arg))
                    {
                        error = "알 수 없는 옵션입니다: " + arg;
                        return false;
                    }
                    continue;
                }
                if (!endOfOptions && arg.Length > 1 && arg[0] == '-')
                {
                    for (int i = 1; i < arg.Length; i++)
                    {
                        if (!ApplyShortOption(arg[i]))
                        {
                            error = "알 수 없는 옵션입니다: -" + arg[i];
                            return false;
                        }
                    }
                    continue;
                }
                Paths.Add(CleanPathArgument(arg));
            }
            return true;
        }

        private bool ApplyLongOption(string option)
        {
            switch (option)
            {
                case "--dry-run": DryRun = true; return true;
                case "--recursive": Recursive = true; return true;
                case "--children": Children = true; return true;
                case "--no-url": Normalize.UrlDecode = false; return true;
                case "--no-nfc": Normalize.Nfc = false; return true;
                case "--gui": Gui = true; return true;
                case "--quiet": Quiet = true; return true;
                case "--verbose": Verbose = true; return true;
                case "--register": Register = true; return true;
                case "--unregister": Unregister = true; return true;
                case "--help": ShowHelp = true; return true;
                case "--version": ShowVersion = true; return true;
                default: return false;
            }
        }

        private bool ApplyShortOption(char option)
        {
            switch (option)
            {
                case 'n': DryRun = true; return true;
                case 'r': Recursive = true; return true;
                case 'c': Children = true; return true;
                case 'q': Quiet = true; return true;
                case 'v': Verbose = true; return true;
                case 'h': ShowHelp = true; return true;
                case 'V': ShowVersion = true; return true;
                default: return false;
            }
        }

        /// <summary>
        /// 탐색기가 "%V"로 드라이브 루트(D:\)를 넘기면 "D:\"의 \" 가 따옴표 이스케이프로 해석되어
        /// 인수가 D:" 로 들어옵니다. 경로에는 따옴표가 올 수 없으므로 끝의 따옴표를 지우고,
        /// "D:"처럼 드라이브 문자만 남으면 루트(D:\)로 되돌립니다("D:"는 현재 폴더를 뜻하므로).
        /// </summary>
        internal static string CleanPathArgument(string raw)
        {
            string path = raw.TrimEnd('"');
            if (path.Length == 2 && path[1] == ':' && char.IsLetter(path[0]))
            {
                path += "\\";
            }
            return path;
        }
    }
}
