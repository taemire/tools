// namefix - Win32 호출
//
// .NET Framework 기본 API로 처리할 수 없는 기능만 모았습니다.
//   - 항목 식별자(볼륨 일련번호·파일 ID) 조회, 식별자로 현재 경로 찾기(OpenFileById + GetFinalPathNameByHandle)
//   - 대기 중인 비동기 입출력 취소(CancelIoEx)
//   - 폴더의 대소문자 구분 여부 조회(FileCaseSensitiveInfo)
//   - 탐색기에 이름 변경 알림(SHChangeNotify)
//   - 대화상자 DPI 인식 설정
using System;
using System.Globalization;
using System.IO;
using System.Runtime.InteropServices;
using System.Text;
using Microsoft.Win32.SafeHandles;

namespace NameFix
{
    internal static class NativeMethods
    {
        private const uint FileReadAttributes = 0x00000080;     // FILE_READ_ATTRIBUTES
        private const uint FileShareAll = 0x00000007;           // FILE_SHARE_READ | WRITE | DELETE
        private const uint OpenExisting = 3;                    // OPEN_EXISTING
        private const uint FileFlagBackupSemantics = 0x02000000; // 폴더 핸들을 열 때 필요합니다.

        private const int FileCaseSensitiveInfoClass = 23;      // FILE_INFO_BY_HANDLE_CLASS.FileCaseSensitiveInfo
        private const uint FileCsFlagCaseSensitiveDir = 0x00000001;

        private const int ShcneRenameItem = 0x00000001;         // SHCNE_RENAMEITEM
        private const int ShcneRenameFolder = 0x00020000;       // SHCNE_RENAMEFOLDER
        private const uint ShcnfPathW = 0x0005;                 // SHCNF_PATHW

        [DllImport("kernel32.dll", EntryPoint = "CreateFileW", CharSet = CharSet.Unicode, SetLastError = true)]
        private static extern SafeFileHandle CreateFile(
            string fileName, uint desiredAccess, uint shareMode, IntPtr securityAttributes,
            uint creationDisposition, uint flagsAndAttributes, IntPtr templateFile);

        [DllImport("kernel32.dll", SetLastError = true)]
        [return: MarshalAs(UnmanagedType.Bool)]
        private static extern bool GetFileInformationByHandleEx(SafeFileHandle file, int informationClass, out uint information, uint bufferSize);

        [StructLayout(LayoutKind.Sequential)]
        private struct ByHandleFileInformation
        {
            public uint FileAttributes;
            public System.Runtime.InteropServices.ComTypes.FILETIME CreationTime;
            public System.Runtime.InteropServices.ComTypes.FILETIME LastAccessTime;
            public System.Runtime.InteropServices.ComTypes.FILETIME LastWriteTime;
            public uint VolumeSerialNumber;
            public uint FileSizeHigh;
            public uint FileSizeLow;
            public uint NumberOfLinks;
            public uint FileIndexHigh;
            public uint FileIndexLow;
        }

        [DllImport("kernel32.dll", SetLastError = true)]
        [return: MarshalAs(UnmanagedType.Bool)]
        private static extern bool GetFileInformationByHandle(SafeFileHandle file, out ByHandleFileInformation information);

        private const uint FileFlagOpenReparsePoint = 0x00200000; // 링크 대상이 아닌 링크 자체를 엽니다.

        // FILE_ID_DESCRIPTOR: DWORD dwSize; FILE_ID_TYPE Type; union { LARGE_INTEGER FileId; GUID; FILE_ID_128 } (총 24바이트)
        [StructLayout(LayoutKind.Explicit, Size = 24)]
        private struct FileIdDescriptor
        {
            [FieldOffset(0)]
            public uint Size;

            [FieldOffset(4)]
            public int Type;        // 0 = FileIdType(64비트 파일 ID)

            [FieldOffset(8)]
            public long FileId;
        }

        [DllImport("kernel32.dll", SetLastError = true)]
        private static extern SafeFileHandle OpenFileById(SafeFileHandle volumeHint, ref FileIdDescriptor fileId,
            uint desiredAccess, uint shareMode, IntPtr securityAttributes, uint flagsAndAttributes);

        [DllImport("kernel32.dll", EntryPoint = "GetFinalPathNameByHandleW", CharSet = CharSet.Unicode, SetLastError = true)]
        private static extern uint GetFinalPathNameByHandle(SafeFileHandle file, StringBuilder path, uint size, uint flags);

        [DllImport("kernel32.dll", SetLastError = true)]
        [return: MarshalAs(UnmanagedType.Bool)]
        private static extern bool CancelIoEx(SafeHandle file, IntPtr overlapped);

        [DllImport("shell32.dll", CharSet = CharSet.Unicode)]
        private static extern void SHChangeNotify(int eventId, uint flags, string item1, string item2);

        [DllImport("user32.dll")]
        [return: MarshalAs(UnmanagedType.Bool)]
        private static extern bool SetProcessDPIAware();

        /// <summary>
        /// 항목의 식별자("볼륨 일련번호-파일 ID")를 돌려줍니다. 이름을 바꾸거나 옮겨도 같은 볼륨 안의 같은 항목이면 값이 같습니다(NTFS 등).
        /// followLinks가 false이면 링크(정션·심볼릭 링크) 자체의 식별자, true이면 링크가 가리키는 실제 항목의 식별자입니다.
        /// 파일 ID를 제공하지 않는 파일 시스템(ID가 0)이나 오류이면 null입니다.
        /// </summary>
        public static string TryGetFileIdentity(string path, bool followLinks = false)
        {
            try
            {
                uint flags = FileFlagBackupSemantics | (followLinks ? 0 : FileFlagOpenReparsePoint);
                using (SafeFileHandle handle = CreateFile(path, FileReadAttributes, FileShareAll, IntPtr.Zero, OpenExisting, flags, IntPtr.Zero))
                {
                    ByHandleFileInformation info;
                    if (handle.IsInvalid || !TryGetInformation(handle, out info))
                    {
                        return null;
                    }
                    return FormatIdentity(info);
                }
            }
            catch (Exception)
            {
                return null;
            }
        }

        /// <summary>
        /// 식별자(TryGetFileIdentity 결과)로 항목을 열어 현재 전체 경로를 돌려줍니다. 항목의 이름이나 상위 폴더 이름이 바뀌어도 찾습니다.
        /// hintPath는 원래 요청 경로이며, 그 경로에서 가장 가까운 기존 상위 폴더로 볼륨을 정합니다(볼륨 일련번호가 같아야 함).
        /// 하드 링크가 둘 이상인 파일은 어느 경로가 요청 항목인지 정할 수 없으므로 null입니다. 찾지 못해도 null입니다.
        /// </summary>
        public static string TryResolveCurrentPath(string identity, string hintPath)
        {
            // 테스트용: NAMEFIX_TEST_DISABLE_OPEN_BY_ID=1이면 OpenFileById를 쓸 수 없는 파일 시스템(SMB 등)처럼 동작합니다.
            if (Environment.GetEnvironmentVariable("NAMEFIX_TEST_DISABLE_OPEN_BY_ID") == "1")
            {
                return null;
            }
            try
            {
                uint serial;
                long fileId;
                if (!TryParseIdentity(identity, out serial, out fileId))
                {
                    return null;
                }
                using (SafeFileHandle hint = OpenVolumeHint(hintPath, serial))
                {
                    if (hint == null)
                    {
                        return null;
                    }
                    FileIdDescriptor descriptor = new FileIdDescriptor();
                    descriptor.Size = 24;
                    descriptor.Type = 0;
                    descriptor.FileId = fileId;
                    using (SafeFileHandle item = OpenFileById(hint, ref descriptor, FileReadAttributes, FileShareAll, IntPtr.Zero,
                        FileFlagBackupSemantics | FileFlagOpenReparsePoint))
                    {
                        ByHandleFileInformation info;
                        if (item.IsInvalid || !TryGetInformation(item, out info) ||
                            !string.Equals(FormatIdentity(info), identity, StringComparison.Ordinal) ||
                            info.NumberOfLinks > 1)
                        {
                            return null;
                        }
                        return GetFinalPath(item);
                    }
                }
            }
            catch (Exception)
            {
                return null;
            }
        }

        /// <summary>
        /// 경로를 링크(정션·심볼릭 링크, 연쇄 포함)를 모두 따라간 실제 전체 경로로 바꿉니다. 얻지 못하면 null입니다.
        /// 처리 대상의 부모 경로를 정션이 없는 실제 경로로 확정해, 다른 대상의 이름 변경에 경로가 끊기지 않게 할 때 씁니다.
        /// </summary>
        public static string TryGetFinalPath(string path)
        {
            try
            {
                using (SafeFileHandle handle = CreateFile(path, FileReadAttributes, FileShareAll, IntPtr.Zero, OpenExisting,
                    FileFlagBackupSemantics, IntPtr.Zero))
                {
                    return handle.IsInvalid ? null : GetFinalPath(handle);
                }
            }
            catch (Exception)
            {
                return null;
            }
        }

        /// <summary>
        /// 항목(링크 자체)의 하드 링크 수를 돌려줍니다. 얻지 못하면 -1입니다. 폴더는 1입니다.
        /// 같은 파일 ID를 가진 디렉터리 항목이 여럿이면(하드 링크) 식별자만으로 요청 항목을 정할 수 없으므로 거부할 때 씁니다.
        /// </summary>
        public static int GetHardLinkCount(string path)
        {
            try
            {
                using (SafeFileHandle handle = CreateFile(path, FileReadAttributes, FileShareAll, IntPtr.Zero, OpenExisting,
                    FileFlagBackupSemantics | FileFlagOpenReparsePoint, IntPtr.Zero))
                {
                    ByHandleFileInformation info;
                    if (handle.IsInvalid || !GetFileInformationByHandle(handle, out info))
                    {
                        return -1;
                    }
                    return (int)info.NumberOfLinks;
                }
            }
            catch (Exception)
            {
                return -1;
            }
        }

        // 열린 핸들의 현재 전체 경로(GetFinalPathNameByHandle)를 일반 Win32 경로로 돌려줍니다. 실패하면 null입니다.
        private static string GetFinalPath(SafeFileHandle handle)
        {
            StringBuilder buffer = new StringBuilder(1024);
            uint length = GetFinalPathNameByHandle(handle, buffer, (uint)buffer.Capacity, 0);
            if (length == 0)
            {
                return null;
            }
            if (length >= buffer.Capacity)
            {
                buffer = new StringBuilder((int)length + 1);
                length = GetFinalPathNameByHandle(handle, buffer, (uint)buffer.Capacity, 0);
                if (length == 0 || length >= buffer.Capacity)
                {
                    return null;
                }
            }
            return ToWin32Path(buffer.ToString());
        }

        // 요청 경로에서 가장 가까운 기존 상위 폴더(자신 포함)를 열고, 그 볼륨이 식별자의 볼륨과 같을 때만 핸들을 돌려줍니다.
        private static SafeFileHandle OpenVolumeHint(string path, uint serial)
        {
            string current = path;
            while (!string.IsNullOrEmpty(current))
            {
                if (Directory.Exists(current))
                {
                    SafeFileHandle handle = CreateFile(current, FileReadAttributes, FileShareAll, IntPtr.Zero, OpenExisting,
                        FileFlagBackupSemantics, IntPtr.Zero);
                    ByHandleFileInformation info;
                    if (!handle.IsInvalid && TryGetInformation(handle, out info) && info.VolumeSerialNumber == serial)
                    {
                        return handle;
                    }
                    handle.Dispose();
                    return null;
                }
                current = Path.GetDirectoryName(current);
            }
            return null;
        }

        private static bool TryGetInformation(SafeFileHandle handle, out ByHandleFileInformation info)
        {
            if (!GetFileInformationByHandle(handle, out info))
            {
                return false;
            }
            return info.FileIndexHigh != 0 || info.FileIndexLow != 0;
        }

        private static string FormatIdentity(ByHandleFileInformation info)
        {
            return info.VolumeSerialNumber.ToString("x8", CultureInfo.InvariantCulture) + "-" +
                   info.FileIndexHigh.ToString("x8", CultureInfo.InvariantCulture) + "-" +
                   info.FileIndexLow.ToString("x8", CultureInfo.InvariantCulture);
        }

        private static bool TryParseIdentity(string identity, out uint serial, out long fileId)
        {
            serial = 0;
            fileId = 0;
            if (string.IsNullOrEmpty(identity))
            {
                return false;
            }
            string[] parts = identity.Split('-');
            uint high;
            uint low;
            if (parts.Length != 3 ||
                !uint.TryParse(parts[0], NumberStyles.AllowHexSpecifier, CultureInfo.InvariantCulture, out serial) ||
                !uint.TryParse(parts[1], NumberStyles.AllowHexSpecifier, CultureInfo.InvariantCulture, out high) ||
                !uint.TryParse(parts[2], NumberStyles.AllowHexSpecifier, CultureInfo.InvariantCulture, out low))
            {
                return false;
            }
            fileId = (long)(((ulong)high << 32) | low);
            return true;
        }

        // GetFinalPathNameByHandle 결과(\\?\D:\..., \\?\UNC\server\share\...)를 일반 Win32 경로로 바꿉니다.
        private static string ToWin32Path(string path)
        {
            if (path.StartsWith(@"\\?\UNC\", StringComparison.OrdinalIgnoreCase))
            {
                return @"\\" + path.Substring(8);
            }
            if (path.StartsWith(@"\\?\", StringComparison.Ordinal))
            {
                return path.Substring(4);
            }
            return path;
        }

        /// <summary>
        /// 핸들에 걸린 대기 중인 비동기 입출력의 취소를 요청합니다.
        /// CancelIoEx는 취소 완료를 기다리지 않습니다. 호출한 쪽은 이어서 End*를 호출해 작업이 끝나기를 기다리고 자원을 회수합니다.
        /// 이미 끝난 작업이면 아무 일도 하지 않습니다.
        /// </summary>
        public static void CancelPendingIo(SafeHandle handle)
        {
            try
            {
                if (handle != null && !handle.IsInvalid && !handle.IsClosed)
                {
                    CancelIoEx(handle, IntPtr.Zero);
                }
            }
            catch (Exception)
            {
                // 취소 실패는 무시합니다. End*는 작업이 끝날 때까지 기다립니다.
            }
        }

        /// <summary>
        /// 폴더가 대소문자를 구분하도록 설정되어 있는지 확인합니다(Windows 10 1803 이상, NTFS).
        /// 확인할 수 없으면 Windows 기본값인 "구분하지 않음"으로 봅니다.
        /// </summary>
        public static bool IsCaseSensitiveDirectory(string directory)
        {
            try
            {
                using (SafeFileHandle handle = CreateFile(directory, FileReadAttributes, FileShareAll, IntPtr.Zero,
                    OpenExisting, FileFlagBackupSemantics, IntPtr.Zero))
                {
                    if (handle.IsInvalid)
                    {
                        return false;
                    }
                    uint flags;
                    if (!GetFileInformationByHandleEx(handle, FileCaseSensitiveInfoClass, out flags, 4))
                    {
                        return false;
                    }
                    return (flags & FileCsFlagCaseSensitiveDir) != 0;
                }
            }
            catch (Exception)
            {
                return false;
            }
        }

        /// <summary>탐색기에 이름 변경을 알려 화면을 바로 갱신하게 합니다. 실패해도 이름 변경 결과에는 영향이 없습니다.</summary>
        public static void NotifyRename(string oldPath, string newPath, bool isDirectory)
        {
            try
            {
                SHChangeNotify(isDirectory ? ShcneRenameFolder : ShcneRenameItem, ShcnfPathW, oldPath, newPath);
            }
            catch (Exception)
            {
                // 알림 실패는 무시합니다. 탐색기는 파일 시스템 변경 감시로도 갱신됩니다.
            }
        }

        /// <summary>고해상도 화면에서 대화상자가 흐리게 보이지 않도록 DPI 인식을 켭니다.</summary>
        public static void TrySetDpiAware()
        {
            try
            {
                SetProcessDPIAware();
            }
            catch (Exception)
            {
                // 지원하지 않는 환경에서는 기본 배율로 표시합니다.
            }
        }
    }
}
