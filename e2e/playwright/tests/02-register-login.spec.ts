import { test, expect } from "@playwright/test";

// 一意のユーザ名を発行 (DB は毎回 fresh だが同一テスト内の重複を避ける目的)
const u = (suffix: string) => `e2e_${suffix}_${Date.now().toString().slice(-6)}`;
const PASSWORD = "P@ssw0rd!Strong";

test.describe("Register / login / logout (Web flow)", () => {
  test("user can register, see quiz page, logout, login again", async ({ page }) => {
    const username = u("alice");
    await page.goto("/register");
    await expect(page.getByRole("heading", { name: "新規登録" })).toBeVisible();
    await page.getByLabel(/ユーザー名/).fill(username);
    await page.getByLabel(/パスワード/).fill(PASSWORD);
    await page.getByRole("button", { name: "登録" }).click();

    // 登録成功 -> /quiz へ
    await expect(page).toHaveURL(/\/quiz/);
    await expect(page.getByRole("heading", { name: "学習" })).toBeVisible();

    // ログアウト
    await page.getByRole("button", { name: "ログアウト" }).click();
    await expect(page).toHaveURL("/");

    // ログイン
    await page.goto("/login");
    await page.getByLabel(/ユーザー名/).fill(username);
    await page.getByLabel(/パスワード/).fill(PASSWORD);
    await page.getByRole("button", { name: "ログイン" }).click();
    await expect(page).toHaveURL(/\/quiz/);
  });

  test("register rejects weak password", async ({ page }) => {
    await page.goto("/register");
    await page.getByLabel(/ユーザー名/).fill(u("weakpw"));
    // Server-side validation: client minlength=12 を上回るが弱い (記号/数字/英大文字なし)
    await page.getByLabel(/パスワード/).fill("aaaaaaaaaaaaaa");
    await page.evaluate(() => {
      // HTML5 minlength を無効化してサーバ検証を露出
      const inputs = document.querySelectorAll<HTMLInputElement>("input[minlength]");
      inputs.forEach((i) => i.removeAttribute("minlength"));
      document.querySelectorAll<HTMLInputElement>("input[pattern]").forEach((i) =>
        i.removeAttribute("pattern"),
      );
    });
    await page.getByRole("button", { name: "登録" }).click();
    await expect(page.getByText(/パスワードは 12 文字以上/)).toBeVisible();
  });

  test("login rejects unknown user (same generic error)", async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel(/ユーザー名/).fill(u("ghost"));
    await page.getByLabel(/パスワード/).fill(PASSWORD);
    await page.getByRole("button", { name: "ログイン" }).click();
    await expect(page.getByText(/ユーザー名またはパスワードが違います/)).toBeVisible();
  });
});
