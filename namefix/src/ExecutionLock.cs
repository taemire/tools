// namefix - 이름 변경 실행 잠금
//
// 같은 사용자의 namefix 실행(탐색기 메뉴, 명령줄, 수집자 비정상 종료 후 다시 처리)이 동시에 같은 항목의 이름을 바꾸면,
// 한쪽이 먼저 바꾼 항목을 다른 쪽이 찾지 못해 오류가 납니다. 실제 이름 변경 단계(대상 수집부터 변경까지)를
// 사용자 단위 프로세스 간 뮤텍스로 직렬화하여, 나중 실행이 앞선 실행의 결과를 보고 처리하게 합니다.
// Global\ 이름을 써서 같은 사용자의 다른 로그온 세션과도 직렬화합니다(이름에 SID를 넣어 사용자별로 분리).
using System;
using System.Security.Principal;
using System.Threading;

namespace NameFix
{
    internal sealed class ExecutionLock : IDisposable
    {
        private Mutex mutex;
        private bool owned;

        private ExecutionLock()
        {
        }

        /// <summary>
        /// 실행 잠금을 얻습니다. 다른 실행이 끝날 때까지 기다립니다.
        /// Global\ 이름을 만들 수 없는 환경(정책 제한 등)에서는 같은 세션 안에서라도 직렬화하도록 Local\ 이름으로 대체합니다.
        /// 둘 다 만들 수 없으면 잠금 없이 진행합니다(반환값은 항상 Dispose 가능).
        /// </summary>
        public static ExecutionLock Acquire()
        {
            ExecutionLock result = new ExecutionLock();
            string sid;
            try
            {
                using (WindowsIdentity identity = WindowsIdentity.GetCurrent())
                {
                    sid = identity.User != null ? identity.User.Value : Environment.UserName;
                }
            }
            catch (Exception)
            {
                sid = Environment.UserName;
            }

            result.mutex = TryCreate("Global\\namefix-execute-" + sid) ?? TryCreate("Local\\namefix-execute-" + sid);
            if (result.mutex == null)
            {
                return result;
            }
            try
            {
                result.owned = result.mutex.WaitOne();
            }
            catch (AbandonedMutexException)
            {
                // 앞선 실행이 비정상 종료했습니다. 소유권은 이 스레드로 넘어왔습니다.
                result.owned = true;
            }
            return result;
        }

        private static Mutex TryCreate(string name)
        {
            try
            {
                return new Mutex(false, name);
            }
            catch (Exception)
            {
                return null;
            }
        }

        public void Dispose()
        {
            if (mutex == null)
            {
                return;
            }
            if (owned)
            {
                try
                {
                    mutex.ReleaseMutex();
                }
                catch (Exception)
                {
                }
                owned = false;
            }
            mutex.Dispose();
            mutex = null;
        }
    }
}
