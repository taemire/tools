// namefix - 다중 실행 요청 집계
//
// 탐색기에서 여러 항목을 선택해 메뉴를 실행하면, 명령줄 방식 메뉴는 항목마다 프로세스를 따로 띄웁니다.
// 그대로 두면 결과 대화상자가 항목 수만큼 뜨므로, 거의 동시에 실행된 인스턴스의 요청을 한 프로세스로 모읍니다.
//
// 동작:
//   1) 먼저 수집자 뮤텍스를 얻은 인스턴스가 "수집자"가 됩니다. 수집자는 자기 수명 뮤텍스를 소유한 채
//      이름 있는 파이프를 열고 다른 인스턴스의 요청을 받습니다.
//   2) 수집자는 마지막 요청 이후 IdleMs 동안 새 요청이 없으면(또는 MaxCollectMs가 지나면) 수집을 끝냅니다.
//   3) 보내는 쪽은 요청을 보내기 전에 자기 완료 이벤트와 문제 이벤트를 만들고, ACK(수집자 수명 뮤텍스 이름 포함)를 받으면
//      "완료 이벤트" 또는 "수집자 수명 뮤텍스(수집자가 끝나면 놓이거나 버려짐)"를 기다립니다.
//   4) 수집자는 이름 정리를 마친 뒤, 받은 요청마다 충돌·오류가 있었으면 문제 이벤트를 먼저 켜고 완료 이벤트를 켭니다.
//      그다음 수명 뮤텍스를 놓습니다.
//   5) 보내는 쪽은 완료 이벤트가 켜졌으면 문제 이벤트에 따라 종료 코드 1 또는 0으로 종료합니다. 완료 신호 없이 수집자가
//      끝났으면(비정상 종료 포함) 같은 요청 ID로 다시 시도합니다. 이때 스스로 수집자가 되어 직접 처리할 수 있습니다.
//
// 처리 보장: 최소 한 번(at-least-once)
//   - 요청은 수집자가 처리를 마쳤다고 알리거나, 보낸 쪽이 직접 처리할 때까지 사라지지 않습니다.
//   - 수집자가 처리 도중 종료되면 같은 요청이 다시 처리될 수 있습니다. 이름 정리를 멱등하게 만들어
//     (Renamer, NameNormalizer 참조) 다시 처리해도 결과가 같고 오류가 나지 않습니다.
//   - 실제 이름 변경 단계는 ExecutionLock으로 사용자 단위 직렬화하므로, 다시 처리가 동시에 겹치지 않습니다.
//   - 파이프 연결·쓰기·읽기 대기에는 모두 제한 시간이 있으며, 시간 초과 시 입출력을 취소하고 완료를 회수합니다.
//     처리 완료 대기에는 제한 시간이 없습니다. 수집자가 살아 있는 동안은 기다리고, 수집자가 끝나면 즉시 깨어납니다.
using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Globalization;
using System.IO;
using System.IO.Pipes;
using System.Security.Principal;
using System.Text;
using System.Threading;

namespace NameFix
{
    /// <summary>
    /// 이 프로세스가 처리할 요청 묶음입니다. 수집자이면 수명 뮤텍스와 완료를 기다리는 요청 ID를 함께 가집니다.
    /// 처리를 마치면 SignalCompleted()로 완료를 알리고, Dispose()로 수명 뮤텍스를 놓습니다(반드시 같은 스레드).
    /// 완료를 알리지 않고 Dispose()하면 기다리던 쪽은 요청을 직접 다시 처리합니다.
    /// </summary>
    internal sealed class CollectorBatch : IDisposable
    {
        public readonly List<WorkItem> Items;

        /// <summary>이 프로세스 자신의 요청 ID(결과의 출처)입니다.</summary>
        public readonly string OwnId;

        private readonly List<string> waitingIds;
        private Mutex lifeMutex;

        public CollectorBatch(List<WorkItem> items, string ownId, Mutex lifeMutex, List<string> waitingIds)
        {
            Items = items;
            OwnId = ownId;
            this.lifeMutex = lifeMutex;
            this.waitingIds = waitingIds;
        }

        /// <summary>다른 프로세스의 요청을 대신 처리하는 수집자인지 여부입니다.</summary>
        public bool IsCollector
        {
            get { return lifeMutex != null; }
        }

        /// <summary>
        /// 받은 요청마다 처리 결과를 알립니다. 충돌·오류가 있었던 요청은 문제 이벤트를 먼저 켜고, 그다음 완료 이벤트를 켭니다.
        /// 보낸 쪽이 이미 종료했으면 건너뜁니다.
        /// </summary>
        public void SignalCompleted(Func<string, bool> hasProblem)
        {
            foreach (string id in waitingIds)
            {
                try
                {
                    if (hasProblem(id))
                    {
                        SetEvent(Collector.ProblemEventName(id));
                    }
                    SetEvent(Collector.DoneEventName(id));
                }
                catch (Exception)
                {
                    // 알리지 못한 쪽은 수명 뮤텍스가 놓인 뒤 직접 다시 처리합니다(멱등).
                }
            }
        }

        private static void SetEvent(string name)
        {
            EventWaitHandle handle;
            if (EventWaitHandle.TryOpenExisting(name, out handle))
            {
                using (handle)
                {
                    handle.Set();
                }
            }
        }

        /// <summary>수명 뮤텍스를 놓습니다. 기다리던 쪽이 깨어납니다.</summary>
        public void Dispose()
        {
            if (lifeMutex != null)
            {
                try
                {
                    lifeMutex.ReleaseMutex();
                }
                catch (Exception)
                {
                }
                lifeMutex.Dispose();
                lifeMutex = null;
            }
        }
    }

    internal static class Collector
    {
        private const byte Ack = 0x06;

        // 요청 ID와 수명 뮤텍스 ID의 길이(GUID "N" 형식, 소문자 16진수 32자)입니다.
        private const int IdLength = 32;

        // 마지막 요청 이후 이 시간 동안 새 요청이 없으면 수집을 끝냅니다.
        private const int DefaultIdleMs = 500;

        // 수집자 한 명이 요청을 받는 최대 시간입니다.
        private const int MaxCollectMs = 15000;

        // 수집에 참여하려고 시도하는 최대 시간입니다. 지나면 자기 요청을 직접 처리합니다.
        private const int MaxJoinMs = MaxCollectMs + 5000;

        // 보내는 쪽이 파이프 연결을 기다리는 시간입니다.
        private const int ConnectTimeoutMs = 1000;

        // 메시지 쓰기·읽기·ACK 대기 시간입니다.
        private const int IoTimeoutMs = 3000;

        // 요청 한 건의 최대 크기입니다(비정상 입력 방지).
        private const int MaxMessageBytes = 4 * 1024 * 1024;

        // 환경 변수 NAMEFIX_COLLECT_MS로 대기 시간을 바꿀 수 있습니다(100~5000ms).
        private static int IdleMs
        {
            get { return ReadIntEnvironment("NAMEFIX_COLLECT_MS", 100, 5000, DefaultIdleMs); }
        }

        // 테스트용: NAMEFIX_TEST_ACK_DELAY_MS가 있으면 수집자가 처음 받은 요청의 ACK를 그만큼 늦게 보냅니다(ACK 시간 초과 재현).
        private static int AckDelayMs
        {
            get { return ReadIntEnvironment("NAMEFIX_TEST_ACK_DELAY_MS", 1, 30000, 0); }
        }

        private static int ReadIntEnvironment(string name, int min, int max, int fallback)
        {
            int value;
            string text = Environment.GetEnvironmentVariable(name);
            if (!string.IsNullOrEmpty(text) &&
                int.TryParse(text, NumberStyles.Integer, CultureInfo.InvariantCulture, out value) &&
                value >= min && value <= max)
            {
                return value;
            }
            return fallback;
        }

        // 같은 사용자·같은 로그온 세션의 인스턴스끼리만 만나도록 SID와 세션 ID를 이름에 넣습니다.
        private static string InstanceSuffix
        {
            get
            {
                string sid;
                using (WindowsIdentity identity = WindowsIdentity.GetCurrent())
                {
                    sid = identity.User != null ? identity.User.Value : Environment.UserName;
                }
                return sid + "-" + Process.GetCurrentProcess().SessionId.ToString(CultureInfo.InvariantCulture);
            }
        }

        /// <summary>요청 ID별 완료 이벤트 이름입니다. 보낸 쪽이 만들고, 수집자가 처리 후 켭니다.</summary>
        internal static string DoneEventName(string requestId)
        {
            return "Local\\namefix-done-" + requestId;
        }

        /// <summary>요청 ID별 문제 이벤트 이름입니다. 그 요청에 충돌·오류가 있었으면 수집자가 완료 이벤트보다 먼저 켭니다.</summary>
        internal static string ProblemEventName(string requestId)
        {
            return "Local\\namefix-problem-" + requestId;
        }

        // 수집자별 수명 뮤텍스 이름입니다. 수집자가 처리를 마치거나 종료하면 놓이거나 버려집니다.
        private static string LifeMutexName(string lifeId)
        {
            return "Local\\namefix-life-" + lifeId;
        }

        /// <summary>
        /// 동시에 실행된 인스턴스의 요청을 모읍니다.
        /// 이 프로세스가 처리할 요청 묶음을 돌려주며, 다른 수집자가 이 요청을 처리 완료했으면 null을 돌려줍니다.
        /// 이때 forwardedExitCode에 그 요청의 결과(0: 정상, 1: 충돌·오류 있음)를 담습니다.
        /// </summary>
        public static CollectorBatch Gather(List<WorkItem> own, out int forwardedExitCode)
        {
            forwardedExitCode = 0;
            string suffix = InstanceSuffix;
            string mutexName = "Local\\namefix-collector-" + suffix;
            string pipeName = "namefix-collector-" + suffix;
            // 재전송 시에도 바뀌지 않는 요청 ID입니다. 같은 수집자가 같은 요청을 두 번 넣지 않게 합니다.
            string requestId = Guid.NewGuid().ToString("N");
            // 결과를 요청별로 돌려줄 수 있도록 자기 요청 항목에 출처를 표시합니다.
            foreach (WorkItem item in own)
            {
                item.Origin = requestId;
            }

            // 완료·문제 이벤트는 요청을 보내기 전에 만들어, 수집자가 처리를 마치는 순간 이미 있게 합니다.
            using (EventWaitHandle done = new EventWaitHandle(false, EventResetMode.ManualReset, DoneEventName(requestId)))
            using (EventWaitHandle problem = new EventWaitHandle(false, EventResetMode.ManualReset, ProblemEventName(requestId)))
            using (Mutex collectorMutex = new Mutex(false, mutexName))
            {
                Stopwatch elapsed = Stopwatch.StartNew();
                while (elapsed.ElapsedMilliseconds < MaxJoinMs)
                {
                    if (TryAcquire(collectorMutex))
                    {
                        // 수집자가 됩니다. 수명 뮤텍스를 소유한 채 수집하고, 처리 후 놓습니다(CollectorBatch.Dispose).
                        Mutex life = new Mutex(true, LifeMutexName(requestId));
                        List<WorkItem> all = new List<WorkItem>(own);
                        List<string> waiting = new List<string>();
                        HashSet<string> receivedIds = new HashSet<string>(StringComparer.Ordinal);
                        receivedIds.Add(requestId);
                        try
                        {
                            Serve(pipeName, requestId, all, receivedIds, waiting);
                        }
                        catch (Exception)
                        {
                            // 수집 중 예기치 않은 오류가 나도 이미 받은 요청은 그대로 처리합니다.
                        }
                        finally
                        {
                            // 파이프를 닫은 뒤 수집자 뮤텍스를 놓습니다. 그 사이에 도착한 인스턴스는 다음 수집자가 됩니다.
                            collectorMutex.ReleaseMutex();
                        }
                        return new CollectorBatch(all, requestId, life, waiting);
                    }

                    string lifeId;
                    if (TrySend(pipeName, requestId, own, out lifeId))
                    {
                        if (WaitForCompletion(done, lifeId))
                        {
                            // 수집자는 문제 이벤트를 완료 이벤트보다 먼저 켜므로, 여기서 확인하면 결과가 확정되어 있습니다.
                            forwardedExitCode = problem.WaitOne(0) ? 1 : 0;
                            return null;
                        }
                        // 완료 신호 없이 수집자가 끝났습니다. 같은 요청 ID로 다시 시도합니다.
                        continue;
                    }
                    Thread.Sleep(30);
                }
            }
            // 수집에 끝내 참여하지 못하면 자기 요청만 직접 처리합니다.
            return new CollectorBatch(new List<WorkItem>(own), requestId, null, new List<string>());
        }

        private static bool TryAcquire(Mutex mutex)
        {
            try
            {
                return mutex.WaitOne(0);
            }
            catch (AbandonedMutexException)
            {
                // 이전 수집자가 비정상 종료한 경우입니다. 소유권은 이 스레드로 넘어왔습니다.
                return true;
            }
        }

        /// <summary>
        /// 수집자가 이 요청을 처리 완료할 때까지 기다립니다. 완료되었으면 true,
        /// 완료 신호 없이 수집자가 끝났으면(수명 뮤텍스가 놓이거나 버려짐, 또는 이미 없음) false를 돌려줍니다.
        /// </summary>
        private static bool WaitForCompletion(EventWaitHandle done, string lifeId)
        {
            Mutex life;
            if (!Mutex.TryOpenExisting(LifeMutexName(lifeId), out life))
            {
                // 수집자가 이미 끝나 수명 뮤텍스가 없어졌습니다. 완료 신호를 남겼는지만 확인합니다.
                return done.WaitOne(0);
            }
            using (life)
            {
                bool ownsLife = false;
                try
                {
                    int index;
                    try
                    {
                        // 완료 이벤트를 앞에 두어, 둘 다 신호 상태이면 완료로 판정되게 합니다.
                        index = WaitHandle.WaitAny(new WaitHandle[] { done, life });
                    }
                    catch (AbandonedMutexException ex)
                    {
                        // 수집자가 비정상 종료했습니다. 버려진 뮤텍스의 소유권은 이 스레드로 넘어왔습니다.
                        index = ex.MutexIndex;
                    }
                    if (index == 0)
                    {
                        return true;
                    }
                    ownsLife = true;
                    return done.WaitOne(0);
                }
                finally
                {
                    if (ownsLife)
                    {
                        try
                        {
                            life.ReleaseMutex();
                        }
                        catch (Exception)
                        {
                        }
                    }
                }
            }
        }

        // 수집자: 요청이 끊길 때까지 파이프 연결을 하나씩 받아 요청을 모읍니다.
        // 연결 하나에서 생긴 오류는 그 연결만 버리고 수집을 계속합니다.
        private static void Serve(string pipeName, string lifeId, List<WorkItem> all, HashSet<string> receivedIds, List<string> waiting)
        {
            int idleMs = IdleMs;
            int ackDelayMs = AckDelayMs;
            byte[] ackMessage = new byte[1 + IdLength];
            ackMessage[0] = Ack;
            Encoding.ASCII.GetBytes(lifeId, 0, IdLength, ackMessage, 1);

            Stopwatch total = Stopwatch.StartNew();
            while (total.ElapsedMilliseconds < MaxCollectMs)
            {
                NamedPipeServerStream server;
                try
                {
                    server = new NamedPipeServerStream(pipeName, PipeDirection.InOut,
                        NamedPipeServerStream.MaxAllowedServerInstances, PipeTransmissionMode.Byte, PipeOptions.Asynchronous);
                }
                catch (Exception)
                {
                    // 파이프를 만들 수 없으면 수집을 끝냅니다. 이미 받은 요청은 호출한 쪽에서 처리합니다.
                    return;
                }

                try
                {
                    IAsyncResult pending = server.BeginWaitForConnection(null, null);
                    if (!pending.AsyncWaitHandle.WaitOne(idleMs))
                    {
                        // 새 요청이 없으므로 수집을 끝냅니다. 대기 중인 연결 작업은 취소하고 완료를 회수합니다.
                        CancelAndComplete(server, pending, delegate(IAsyncResult r) { server.EndWaitForConnection(r); });
                        return;
                    }
                    server.EndWaitForConnection(pending);

                    string id;
                    List<WorkItem> received;
                    if (!ReadRequest(server, out id, out received))
                    {
                        continue;
                    }

                    // 끝까지 읽은 요청은 ACK 전송 결과와 관계없이 즉시 넣습니다. 같은 ID의 재전송은 넣지 않고 ACK만 보냅니다.
                    if (receivedIds.Add(id))
                    {
                        // 결과를 요청별로 돌려줄 수 있도록 받은 항목에 보낸 쪽의 요청 ID를 출처로 표시합니다.
                        foreach (WorkItem item in received)
                        {
                            item.Origin = id;
                        }
                        all.AddRange(received);
                        waiting.Add(id);
                        if (ackDelayMs > 0)
                        {
                            Thread.Sleep(ackDelayMs);
                        }
                    }

                    if (!WriteExact(server, ackMessage, IoTimeoutMs))
                    {
                        continue;
                    }

                    // 보낸 쪽이 ACK를 읽고 연결을 닫을 때까지 잠시 기다립니다.
                    byte[] hangup = new byte[1];
                    ReadExact(server, hangup, 0, 1, IoTimeoutMs);
                }
                catch (Exception)
                {
                    // 연결 직후 끊김(ERROR_NO_DATA 등)은 이 연결만 버리고 계속합니다.
                }
                finally
                {
                    server.Dispose();
                }
            }
        }

        // 보내는 쪽: 요청을 보내고 ACK를 받으면 true와 수집자의 수명 뮤텍스 ID를 돌려줍니다.
        private static bool TrySend(string pipeName, string requestId, List<WorkItem> items, out string lifeId)
        {
            lifeId = null;
            try
            {
                using (NamedPipeClientStream client = new NamedPipeClientStream(".", pipeName, PipeDirection.InOut, PipeOptions.Asynchronous))
                {
                    client.Connect(ConnectTimeoutMs);
                    byte[] payload = Encoding.UTF8.GetBytes(requestId + "\n" + WorkItem.SerializeMany(items));
                    byte[] message = new byte[4 + payload.Length];
                    Buffer.BlockCopy(BitConverter.GetBytes(payload.Length), 0, message, 0, 4);
                    Buffer.BlockCopy(payload, 0, message, 4, payload.Length);
                    if (!WriteExact(client, message, IoTimeoutMs))
                    {
                        return false;
                    }

                    byte[] answer = new byte[1 + IdLength];
                    if (!ReadExact(client, answer, 0, answer.Length, IoTimeoutMs) || answer[0] != Ack)
                    {
                        return false;
                    }
                    string id = Encoding.ASCII.GetString(answer, 1, IdLength);
                    if (!IsValidId(id))
                    {
                        return false;
                    }
                    lifeId = id;
                    return true;
                }
            }
            catch (Exception)
            {
                // 연결 시간 초과, 파이프 끊김 등은 모두 "넘기지 못함"으로 처리하고 같은 ID로 다시 시도합니다.
                return false;
            }
        }

        // 요청 메시지를 읽습니다. 형식: 4바이트 길이(리틀 엔디언) + UTF-8 본문("<요청 ID>\n<요청 줄>...").
        private static bool ReadRequest(PipeStream stream, out string id, out List<WorkItem> items)
        {
            id = null;
            items = null;
            byte[] header = new byte[4];
            if (!ReadExact(stream, header, 0, 4, IoTimeoutMs))
            {
                return false;
            }
            int length = BitConverter.ToInt32(header, 0);
            if (length <= 0 || length > MaxMessageBytes)
            {
                return false;
            }
            byte[] payload = new byte[length];
            if (!ReadExact(stream, payload, 0, length, IoTimeoutMs))
            {
                return false;
            }

            string[] lines = Encoding.UTF8.GetString(payload).Split('\n');
            if (!IsValidId(lines[0]))
            {
                return false;
            }
            id = lines[0];
            string[] rest = new string[lines.Length - 1];
            Array.Copy(lines, 1, rest, 0, rest.Length);
            items = WorkItem.ParseMany(rest);
            return true;
        }

        private static bool IsValidId(string id)
        {
            if (string.IsNullOrEmpty(id) || id.Length != IdLength)
            {
                return false;
            }
            foreach (char c in id)
            {
                if (!((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')))
                {
                    return false;
                }
            }
            return true;
        }

        // 제한 시간 안에 버퍼 전체를 씁니다. 상대가 읽지 않아 시간을 넘기면 쓰기를 취소하고 false를 돌려줍니다.
        private static bool WriteExact(PipeStream stream, byte[] buffer, int timeoutMs)
        {
            try
            {
                IAsyncResult pending = stream.BeginWrite(buffer, 0, buffer.Length, null, null);
                if (!pending.AsyncWaitHandle.WaitOne(timeoutMs))
                {
                    CancelAndComplete(stream, pending, delegate(IAsyncResult r) { stream.EndWrite(r); });
                    return false;
                }
                stream.EndWrite(pending);
                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }

        // 제한 시간 안에 정확히 count바이트를 읽습니다. 시간 초과·연결 종료·오류면 false를 돌려줍니다.
        private static bool ReadExact(PipeStream stream, byte[] buffer, int offset, int count, int timeoutMs)
        {
            try
            {
                Stopwatch watch = Stopwatch.StartNew();
                while (count > 0)
                {
                    int remaining = timeoutMs - (int)watch.ElapsedMilliseconds;
                    if (remaining <= 0)
                    {
                        return false;
                    }
                    IAsyncResult pending = stream.BeginRead(buffer, offset, count, null, null);
                    if (!pending.AsyncWaitHandle.WaitOne(remaining))
                    {
                        CancelAndComplete(stream, pending, delegate(IAsyncResult r) { stream.EndRead(r); });
                        return false;
                    }
                    int read = stream.EndRead(pending);
                    if (read <= 0)
                    {
                        return false;
                    }
                    offset += read;
                    count -= read;
                }
                return true;
            }
            catch (Exception)
            {
                return false;
            }
        }

        /// <summary>
        /// 시간을 넘긴 비동기 작업을 포기합니다. 핸들이 열린 상태에서 입출력 취소를 요청(CancelIoEx)한 뒤 End*를 정확히 한 번 호출합니다.
        /// End*는 작업이 끝날 때까지(취소 완료 포함) 기다린 뒤 오버랩 구조체와 대기 핸들을 회수합니다.
        /// 취소된 작업의 End*는 예외를 던지며, 자원 회수가 목적이므로 무시합니다. 스트림은 호출한 쪽이 닫습니다.
        /// </summary>
        private static void CancelAndComplete(PipeStream stream, IAsyncResult pending, Action<IAsyncResult> end)
        {
            try
            {
                NativeMethods.CancelPendingIo(stream.SafePipeHandle);
            }
            catch (Exception)
            {
            }
            try
            {
                end(pending);
            }
            catch (Exception)
            {
            }
        }

        /// <summary>
        /// 테스트용 일시 정지 지점입니다. 환경 변수 NAMEFIX_TEST_PAUSE_MS(1~60000)가 있으면 요청 경로와 식별자를 확정한 뒤,
        /// 집계·처리 전에 그만큼 멈춥니다. 요청 시점과 처리 시점 사이에 항목이 바뀌는 상황을 테스트에서 재현할 때 씁니다.
        /// </summary>
        public static void TestPause()
        {
            int pauseMs = ReadIntEnvironment("NAMEFIX_TEST_PAUSE_MS", 1, 60000, 0);
            if (pauseMs > 0)
            {
                Thread.Sleep(pauseMs);
            }
        }

        /// <summary>
        /// 테스트용 비정상 종료 지점입니다. 수집자에서만, 환경 변수 NAMEFIX_TEST_CRASH_POINT가 point와 같고
        /// NAMEFIX_TEST_CRASH_MARKER 경로에 표시 파일을 새로 만들 수 있을 때 한 번만 프로세스를 강제 종료합니다.
        /// 표시 파일이 이미 있으면(두 번째 수집자부터) 아무 일도 하지 않습니다.
        /// </summary>
        public static void TestCrashPoint(string point, CollectorBatch batch)
        {
            if (batch == null || !batch.IsCollector ||
                !string.Equals(Environment.GetEnvironmentVariable("NAMEFIX_TEST_CRASH_POINT"), point, StringComparison.Ordinal))
            {
                return;
            }
            string marker = Environment.GetEnvironmentVariable("NAMEFIX_TEST_CRASH_MARKER");
            if (string.IsNullOrEmpty(marker))
            {
                return;
            }
            try
            {
                using (new FileStream(marker, FileMode.CreateNew, FileAccess.Write))
                {
                }
            }
            catch (Exception)
            {
                return;
            }
            Process.GetCurrentProcess().Kill();
        }
    }
}
