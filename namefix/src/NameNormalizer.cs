// namefix - 이름 문자열 변환 로직
//
// 이 파일은 이름 문자열만 다룹니다. 파일 시스템에는 접근하지 않습니다.
// 변환 순서는 다음과 같습니다.
//   1) URL 인코딩(%XX) 디코딩: UTF-8 바이트 시퀀스로 해석할 수 있는 부분만 풉니다.
//   2) 유니코드 NFC 정규화: macOS 등에서 넘어온 자소 분리(NFD) 이름을 완성형으로 합칩니다.
// URL 디코딩을 먼저 하는 이유는, 인코딩된 바이트 자체가 NFD 자모일 수 있기 때문입니다
// (예: macOS에서 만든 이름을 브라우저가 %E1%84%8C... 형태로 저장한 경우).
using System;
using System.Text;

namespace NameFix
{
    /// <summary>이름 변환 옵션입니다.</summary>
    internal sealed class NormalizeOptions
    {
        /// <summary>%XX 형식의 URL 인코딩을 UTF-8 기준으로 디코딩할지 여부입니다.</summary>
        public bool UrlDecode = true;

        /// <summary>유니코드 NFC(완성형)로 정규화하여 자소 분리(NFD) 이름을 합칠지 여부입니다.</summary>
        public bool Nfc = true;
    }

    /// <summary>파일·폴더 이름 한 개를 정리된 이름으로 변환합니다.</summary>
    internal static class NameNormalizer
    {
        // NTFS 파일 이름 한 개의 최대 길이(UTF-16 코드 단위)입니다.
        private const int MaxNameLength = 255;

        // Windows 파일 이름에 쓸 수 없는 문자입니다.
        // 디코딩 결과가 이 문자가 되면 해당 %XX는 원문 그대로 남깁니다(예: %2F는 '/'가 되므로 유지).
        private const string ForbiddenChars = "\\/:*?\"<>|";

        // Win32에서 장치 이름으로 예약된 이름입니다. 확장자가 붙어도(CON.txt) 예약어로 취급합니다.
        // 출처: https://learn.microsoft.com/windows/win32/fileio/naming-a-file (2024-08-28 판)
        // 숫자는 1~9와 위 첨자 숫자(¹ ² ³, U+00B9 U+00B2 U+00B3)입니다. COM0·LPT0은 목록에 없으므로 유효한 이름입니다.
        private static readonly string[] ReservedNames =
        {
            "CON", "PRN", "AUX", "NUL",
            "COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
            "COM¹", "COM²", "COM³",
            "LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
            "LPT¹", "LPT²", "LPT³"
        };

        // 전체 변환(디코딩·유효성 검사·NFC)을 고정점까지 반복하는 최대 횟수입니다.
        // 실제 이름은 2~3회 안에 고정점에 도달합니다. 이 한도는 비정상 입력에서 반복을 끝내기 위한 안전장치입니다.
        private const int MaxNormalizePasses = 16;

        /// <summary>
        /// 이름 한 개를 정리합니다. 바꿀 것이 없으면 입력과 같은 문자열을 돌려줍니다.
        /// URL 디코딩 결과가 Windows에서 쓸 수 없는 이름(끝 공백, 예약어 등)이면
        /// URL 디코딩은 적용하지 않고 NFC 정규화만 적용합니다.
        /// 한 번의 변환 결과를 다시 변환하면 더 바뀔 수 있으므로(예: NFC가 결합 문자를 합쳐 디코딩 가능 여부가 달라지는 경우)
        /// 전체 변환을 고정점까지 반복합니다. 따라서 Normalize(Normalize(x)) == Normalize(x)입니다.
        /// </summary>
        public static string Normalize(string name, NormalizeOptions options)
        {
            if (string.IsNullOrEmpty(name) || options == null)
            {
                return name;
            }

            string current = name;
            for (int pass = 0; pass < MaxNormalizePasses; pass++)
            {
                string next = NormalizeOnce(current, options);
                if (string.Equals(next, current, StringComparison.Ordinal))
                {
                    return current;
                }
                current = next;
            }
            return current;
        }

        // 디코딩·유효성 검사·NFC를 한 번 적용합니다.
        private static string NormalizeOnce(string name, NormalizeOptions options)
        {
            string result = name;

            if (options.UrlDecode)
            {
                string decoded = DecodeRepeatedly(result);
                if (!string.Equals(decoded, result, StringComparison.Ordinal))
                {
                    // 최종적으로 저장될 형태(NFC 적용 후)로 유효성을 판정합니다.
                    string candidate = options.Nfc ? ToNfc(decoded) : decoded;
                    if (IsValidFileName(candidate))
                    {
                        result = decoded;
                    }
                }
            }

            if (options.Nfc)
            {
                result = ToNfc(result);
            }

            return result;
        }

        /// <summary>
        /// 더 이상 바뀌지 않을 때까지(고정점) URL 디코딩을 반복합니다. 이중·다중 인코딩(%25ED%2595...)을 끝까지 풉니다.
        /// 디코딩이 일어나면 "%XX" 세 글자가 UTF-16 한두 글자로 줄어 문자열이 짧아지므로 반복은 반드시 끝납니다.
        /// </summary>
        public static string DecodeRepeatedly(string text)
        {
            string current = text;
            while (true)
            {
                string next = DecodeOnce(current);
                if (string.Equals(next, current, StringComparison.Ordinal))
                {
                    return current;
                }
                current = next;
            }
        }

        /// <summary>
        /// %XX 시퀀스를 한 번 디코딩합니다.
        /// 연속된 %XX를 UTF-8 바이트로 보고 한 문자씩 해석하며, 다음 경우에는 해당 %XX 원문을 유지합니다.
        ///   - UTF-8로 올바르지 않은 바이트(잘못된 선행 바이트, 끊긴 시퀀스, 과잉 표현, 서로게이트 범위)
        ///   - 디코딩한 문자가 제어 문자이거나 파일 이름에 쓸 수 없는 문자인 경우
        /// '+'는 공백으로 바꾸지 않습니다. 파일 이름에서는 '+'가 실제 문자로 쓰이는 경우가 많기 때문입니다.
        /// </summary>
        public static string DecodeOnce(string text)
        {
            if (string.IsNullOrEmpty(text) || text.IndexOf('%') < 0)
            {
                return text;
            }

            StringBuilder sb = new StringBuilder(text.Length);
            int i = 0;
            while (i < text.Length)
            {
                int lead = ReadHexByte(text, i);
                if (lead < 0)
                {
                    // %XX 형식이 아닌 일반 문자입니다.
                    sb.Append(text[i]);
                    i++;
                    continue;
                }

                // 선행 바이트로 UTF-8 시퀀스 길이(뒤따르는 바이트 수)를 정합니다.
                int extra;
                int codePoint;
                if (lead <= 0x7F)
                {
                    extra = 0;
                    codePoint = lead;
                }
                else if (lead >= 0xC2 && lead <= 0xDF)
                {
                    extra = 1;
                    codePoint = lead & 0x1F;
                }
                else if (lead >= 0xE0 && lead <= 0xEF)
                {
                    extra = 2;
                    codePoint = lead & 0x0F;
                }
                else if (lead >= 0xF0 && lead <= 0xF4)
                {
                    extra = 3;
                    codePoint = lead & 0x07;
                }
                else
                {
                    // 0x80~0xC1, 0xF5~0xFF는 선행 바이트가 될 수 없습니다. 원문을 유지합니다.
                    sb.Append(text, i, 3);
                    i += 3;
                    continue;
                }

                int next = i + 3;
                bool valid = true;
                for (int k = 0; k < extra; k++)
                {
                    int cont = ReadHexByte(text, next);
                    if (cont < 0 || (cont & 0xC0) != 0x80)
                    {
                        valid = false;
                        break;
                    }
                    codePoint = (codePoint << 6) | (cont & 0x3F);
                    next += 3;
                }

                if (valid)
                {
                    valid = IsWellFormed(codePoint, extra) && IsAllowedChar(codePoint);
                }

                if (!valid)
                {
                    // 선행 %XX 하나만 원문으로 남기고 다음 위치부터 다시 해석합니다.
                    // 뒤따르던 바이트들은 각각 잘못된 선행 바이트로 판정되어 원문이 유지됩니다.
                    sb.Append(text, i, 3);
                    i += 3;
                    continue;
                }

                sb.Append(char.ConvertFromUtf32(codePoint));
                i = next;
            }
            return sb.ToString();
        }

        /// <summary>
        /// NFC로 정규화합니다. 짝이 맞지 않는 서로게이트처럼 정규화할 수 없는 문자열은 그대로 돌려줍니다.
        /// </summary>
        public static string ToNfc(string text)
        {
            if (string.IsNullOrEmpty(text))
            {
                return text;
            }
            try
            {
                return text.IsNormalized(NormalizationForm.FormC) ? text : text.Normalize(NormalizationForm.FormC);
            }
            catch (ArgumentException)
            {
                return text;
            }
        }

        /// <summary>
        /// Windows(Win32)에서 파일·폴더 이름으로 쓸 수 있는지 판정합니다.
        /// 빈 이름, 255자 초과, 금지 문자·제어 문자, 끝의 공백·마침표, 예약 장치 이름을 거부합니다.
        /// </summary>
        public static bool IsValidFileName(string name)
        {
            if (string.IsNullOrEmpty(name) || name.Length > MaxNameLength)
            {
                return false;
            }
            if (name == "." || name == "..")
            {
                return false;
            }
            foreach (char c in name)
            {
                if (c < 0x20 || ForbiddenChars.IndexOf(c) >= 0)
                {
                    return false;
                }
            }

            char last = name[name.Length - 1];
            if (last == ' ' || last == '.')
            {
                return false;
            }

            // 첫 마침표 앞부분(끝 공백 제외)이 예약어이면 거부합니다. 예: "CON.txt", "nul .log"
            int dot = name.IndexOf('.');
            string stem = (dot >= 0 ? name.Substring(0, dot) : name).TrimEnd(' ');
            foreach (string reserved in ReservedNames)
            {
                if (string.Equals(stem, reserved, StringComparison.OrdinalIgnoreCase))
                {
                    return false;
                }
            }
            return true;
        }

        // 과잉 표현(overlong), 서로게이트 영역, 유니코드 범위 초과를 거부합니다.
        // 2바이트 시퀀스의 과잉 표현은 선행 바이트 범위(0xC2 이상)에서 이미 걸러집니다.
        private static bool IsWellFormed(int codePoint, int extra)
        {
            if (extra == 2 && codePoint < 0x800)
            {
                return false;
            }
            if (extra == 3 && (codePoint < 0x10000 || codePoint > 0x10FFFF))
            {
                return false;
            }
            if (codePoint >= 0xD800 && codePoint <= 0xDFFF)
            {
                return false;
            }
            return true;
        }

        // 디코딩 결과로 받아들일 수 있는 문자인지 판정합니다(제어 문자·파일 이름 금지 문자 제외).
        private static bool IsAllowedChar(int codePoint)
        {
            if (codePoint < 0x20 || codePoint == 0x7F)
            {
                return false;
            }
            if (codePoint >= 0x80 && codePoint <= 0x9F)
            {
                return false;
            }
            if (codePoint < 0x80 && ForbiddenChars.IndexOf((char)codePoint) >= 0)
            {
                return false;
            }
            return true;
        }

        // text[index]부터 "%XX" 형식이면 바이트 값(0~255)을, 아니면 -1을 돌려줍니다.
        private static int ReadHexByte(string text, int index)
        {
            if (index < 0 || index + 2 >= text.Length || text[index] != '%')
            {
                return -1;
            }
            int hi = HexValue(text[index + 1]);
            int lo = HexValue(text[index + 2]);
            if (hi < 0 || lo < 0)
            {
                return -1;
            }
            return (hi << 4) | lo;
        }

        private static int HexValue(char c)
        {
            if (c >= '0' && c <= '9')
            {
                return c - '0';
            }
            if (c >= 'a' && c <= 'f')
            {
                return c - 'a' + 10;
            }
            if (c >= 'A' && c <= 'F')
            {
                return c - 'A' + 10;
            }
            return -1;
        }
    }
}
