import { test, expect } from "@playwright/test";

test.describe("Smoke (any visitor)", () => {
  test("home page renders 和モダン hero", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { name: /日商簿記3級.*紙のリズム/ })).toBeVisible();
    await expect(page.getByRole("link", { name: "ログイン" })).toBeVisible();
    await expect(page.getByRole("link", { name: "新規登録" })).toBeVisible();
  });

  test("healthz returns ok", async ({ request }) => {
    const r = await request.get("/healthz");
    expect(r.status()).toBe(200);
    expect((await r.text()).trim()).toBe("ok");
  });

  test("version returns build version", async ({ request }) => {
    const r = await request.get("/version");
    expect(r.status()).toBe(200);
    expect((await r.text()).trim().length).toBeGreaterThan(0);
  });

  test("security headers are set on /", async ({ request }) => {
    const r = await request.get("/");
    expect(r.headers()["content-security-policy"]).toContain("default-src 'self'");
    expect(r.headers()["x-frame-options"]).toBe("DENY");
    expect(r.headers()["strict-transport-security"]).toContain("max-age");
    expect(r.headers()["referrer-policy"]).toBe("strict-origin-when-cross-origin");
  });

  test("/quiz redirects to /login when not authenticated", async ({ page }) => {
    await page.goto("/quiz");
    await expect(page).toHaveURL(/\/login$/);
    await expect(page.getByRole("heading", { name: "ログイン" })).toBeVisible();
  });
});
