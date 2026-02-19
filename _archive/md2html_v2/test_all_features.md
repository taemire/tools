# MD 확장 구문 전체 테스트

## 1. Callouts (Phase 1)

> [!NOTE]
> 이것은 NOTE 알림입니다.

> [!TIP]
> 이것은 TIP 알림입니다.

> [!CAUTION]
> 이것은 CAUTION 알림입니다.

---

## 2. Highlight & Emoji (Phase 2)

이것은 ==하이라이트된 텍스트==입니다.

:rocket: 로켓 + :fire: 불꽃 + :star: 별

---

## 3. Footnotes (Phase 3)

이것은 각주가 포함된 문장입니다[^1]. 여러 개의 각주를 사용할 수 있습니다[^2].

[^1]: 첫 번째 각주의 내용입니다.
[^2]: 두 번째 각주의 내용입니다. 각주는 문서 하단에 자동으로 정리됩니다.

---

## 4. Definition Lists (Phase 3)

API
: Application Programming Interface의 약자입니다.

REST
: Representational State Transfer의 약자입니다.
: RESTful API 설계 원칙을 나타냅니다.

Goldmark
: Go 언어로 작성된 Markdown 파서입니다.

---

## 5. 종합 테스트

> [!TIP]
> :bulb: ==중요==: 각주[^3]와 정의 목록을 함께 사용할 수 있습니다!

[^3]: 이것은 조합 테스트용 각주입니다.

SDK
: Software Development Kit
: :package: 개발 도구 모음

