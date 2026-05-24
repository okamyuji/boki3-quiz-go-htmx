import { test, expect } from "@playwright/test";

/**
 * 全ページに必須セキュリティヘッダが揃っているかを横断的に検証する。
 *
 * 対象:
 *   - X-Frame-Options    : DENY
 *   - Permissions-Policy : camera/microphone/geolocation off
 *   - Content-Security-Policy : default-src 'self' + script nonce
 *   - Referrer-Policy    : strict-origin-when-cross-origin
 *   - X-Content-Type-Options : nosniff
 *   - Strict-Transport-Security : max-age + includeSubDomains
 */

const paths = [
  "/",
  "/healthz",
  "/version",
  "/login",
  "/register",
  "/static/css/app.css",
];

test.describe("Security headers (HTTP)", () => {
  for (const p of paths) {
    test(`headers present on ${p}`, async ({ request }) => {
      const r = await request.get(p);
      // 一部 path (/static/...) は 200 を期待、それ以外も 200/3xx であること
      expect(r.status(), `status of ${p}`).toBeLessThan(400);
      const h = r.headers();

      expect(h["x-frame-options"], `X-Frame-Options on ${p}`).toBe("DENY");
      expect(h["x-content-type-options"], `X-Content-Type-Options on ${p}`).toBe("nosniff");
      expect(h["referrer-policy"], `Referrer-Policy on ${p}`).toBe(
        "strict-origin-when-cross-origin",
      );
      expect(h["permissions-policy"], `Permissions-Policy on ${p}`).toContain("camera=()");
      expect(h["permissions-policy"], `Permissions-Policy on ${p}`).toContain("microphone=()");
      expect(h["permissions-policy"], `Permissions-Policy on ${p}`).toContain("geolocation=()");
      expect(h["strict-transport-security"], `HSTS on ${p}`).toMatch(/max-age=\d+/);
      expect(h["strict-transport-security"], `HSTS on ${p}`).toContain("includeSubDomains");
      expect(h["content-security-policy"], `CSP on ${p}`).toContain("default-src 'self'");
      expect(h["content-security-policy"], `CSP on ${p}`).toContain("nonce-");
    });
  }

  test("authenticated page (/quiz) also carries headers (after register)", async ({ page, request }) => {
    const username = `e2e_sec_${Date.now().toString().slice(-6)}`;
    await page.goto("/register");
    await page.getByLabel(/ユーザー名/).fill(username);
    await page.getByLabel(/パスワード/).fill("P@ssw0rd!Strong");
    await page.getByRole("button", { name: "登録" }).click();
    await expect(page).toHaveURL(/\/quiz/);

    // page.context() の cookies で API 経路へ持ち込む
    const cookies = await page.context().cookies();
    const cookieHeader = cookies.map((c) => `${c.name}=${c.value}`).join("; ");
    const r = await request.get("/quiz", { headers: { Cookie: cookieHeader } });
    const h = r.headers();
    expect(h["x-frame-options"]).toBe("DENY");
    expect(h["content-security-policy"]).toContain("default-src 'self'");
  });
});
