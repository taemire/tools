// index.js - Outlook Web Crawler with Manual Auth Wait Flow
const { chromium } = require('playwright');
require('dotenv').config();

const EMAIL = process.env.OUTLOOK_EMAIL || '';
const PASSWORD = process.env.OUTLOOK_PASSWORD || '';
const AUTH_TIMEOUT_MS = parseInt(process.env.AUTH_TIMEOUT_MS) || 300000; // 5분

/**
 * MFA 대기 안내 메시지 출력
 */
function printAuthWaitMessage() {
    console.log(`
╔══════════════════════════════════════════════════════════╗
║  🔐 2단계 인증이 필요합니다.                              ║
║                                                          ║
║  열린 브라우저 창에서 인증을 완료해주세요.               ║
║  (OTP 입력, 앱 승인, SMS 코드 등)                        ║
║                                                          ║
║  인증 완료 후 메일함이 로딩되면 자동으로 진행됩니다.     ║
║  대기 시간: 최대 ${Math.floor(AUTH_TIMEOUT_MS / 60000)}분                                     ║
╚══════════════════════════════════════════════════════════╝
`);
}

/**
 * 메인 크롤러 함수
 */
(async () => {
    if (!EMAIL) {
        console.error('❌ 오류: .env 파일에 OUTLOOK_EMAIL을 설정해주세요.');
        process.exit(1);
    }

    // 1. 브라우저 실행 (headless: false 필수 - 수동 인증을 위해)
    const browser = await chromium.launch({
        headless: false,
        slowMo: 300
    });

    // 세션 파일이 있으면 복원 시도
    let context;
    const fs = require('fs');
    if (fs.existsSync('session.json')) {
        console.log('📁 기존 세션 발견. 복원 시도 중...');
        context = await browser.newContext({ storageState: 'session.json' });
    } else {
        context = await browser.newContext();
    }

    const page = await context.newPage();

    try {
        console.log('🚀 Outlook 접속 중...');
        await page.goto('https://outlook.office.com/mail/', { waitUntil: 'domcontentloaded' });

        // 2. 이미 로그인된 상태인지 확인
        const isAlreadyLoggedIn = await page.waitForSelector(
            'div[aria-label="Message list"], div[role="listbox"]',
            { timeout: 5000 }
        ).then(() => true).catch(() => false);

        if (isAlreadyLoggedIn) {
            console.log('✅ 기존 세션으로 자동 로그인 완료!');
        } else {
            // 3. 로그인 프로세스
            console.log('🔑 로그인 시작...');

            // 이메일 입력
            const emailInput = await page.waitForSelector('input[type="email"]', { timeout: 10000 });
            if (emailInput) {
                await page.fill('input[type="email"]', EMAIL);
                await page.click('input[type="submit"]');
            }

            // 패스워드 입력
            const pwdInput = await page.waitForSelector('input[type="password"]', { timeout: 10000 });
            if (pwdInput && PASSWORD) {
                await page.fill('input[type="password"]', PASSWORD);
                await page.click('input[type="submit"]');
            }

            // 4. 🖐️ 수동 인증 대기 모드
            printAuthWaitMessage();

            // 메일함이 로딩될 때까지 대기 (최대 AUTH_TIMEOUT_MS)
            await page.waitForSelector(
                'div[aria-label="Message list"], div[role="listbox"]',
                { timeout: AUTH_TIMEOUT_MS }
            );

            console.log('✅ 인증 완료! 메일함 진입 성공.');

            // 5. 세션 저장 (다음번 로그인 생략 가능)
            await context.storageState({ path: 'session.json' });
            console.log('💾 세션 저장 완료 (session.json)');
        }

        // 6. 메일 목록 스크래핑
        console.log('📩 메일 목록 수집 중...');
        await page.waitForTimeout(2000); // 렌더링 안정화 대기

        const emails = await page.evaluate(() => {
            const items = document.querySelectorAll('div[role="option"], div[data-convid]');
            const results = [];

            items.forEach((item, index) => {
                const text = item.innerText;
                const lines = text.split('\n').filter(line => line.trim() !== '');

                results.push({
                    index: index + 1,
                    sender: lines[0] || '(알 수 없음)',
                    subject: lines[1] || '(제목 없음)',
                    preview: lines[2] || '',
                    date: lines[3] || ''
                });
            });
            return results;
        });

        console.log(`\n✨ 총 ${emails.length}개의 메일 발견:\n`);
        console.table(emails.slice(0, 20)); // 상위 20개만 표시

        // 7. 결과 저장
        const outputDir = './output';
        if (!fs.existsSync(outputDir)) fs.mkdirSync(outputDir);

        fs.writeFileSync(`${outputDir}/emails.json`, JSON.stringify(emails, null, 2), 'utf-8');
        console.log(`\n📄 결과 저장 완료: ${outputDir}/emails.json`);

        await page.screenshot({ path: `${outputDir}/inbox.png`, fullPage: false });
        console.log(`📸 스크린샷 저장 완료: ${outputDir}/inbox.png`);

    } catch (error) {
        console.error('❌ 에러 발생:', error.message);
        await page.screenshot({ path: 'error_screenshot.png' });
    } finally {
        console.log('\nℹ️ 브라우저를 닫으려면 아무 키나 누르세요...');
        process.stdin.setRawMode(true);
        process.stdin.resume();
        process.stdin.once('data', async () => {
            await browser.close();
            process.exit(0);
        });
    }
})();
