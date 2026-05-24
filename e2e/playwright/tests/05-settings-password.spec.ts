import { test, expect } from "@playwright/test";

const PASSWORD = "P@ssw0rd!Strong";
const NEW_PASSWORD = "N3wP@ssw0rd!Strong";

test.describe("Password change in settings", () => {
  test("changing password logs other sessions out and accepts new credential", async ({
    browser,
  }) => {
    const username = `e2e_pw_${Date.now().toString().slice(-6)}`;

    // Session A で登録 + ログイン
    const ctxA = await browser.newContext();
    const pageA = await ctxA.newPage();
    await pageA.goto("/register");
    await pageA.getByLabel(/ユーザー名/).fill(username);
    await pageA.getByLabel(/パスワード/).fill(PASSWORD);
    await pageA.getByRole("button", { name: "登録" }).click();
    await expect(pageA).toHaveURL(/\/quiz/);

    // Session B でもログイン (同じユーザー)
    const ctxB = await browser.newContext();
    const pageB = await ctxB.newPage();
    await pageB.goto("/login");
    await pageB.getByLabel(/ユーザー名/).fill(username);
    await pageB.getByLabel(/パスワード/).fill(PASSWORD);
    await pageB.getByRole("button", { name: "ログイン" }).click();
    await expect(pageB).toHaveURL(/\/quiz/);

    // Session A でパスワード変更
    await pageA.goto("/settings");
    await pageA.locator("input[name='current']").fill(PASSWORD);
    await pageA.locator("input[name='new']").fill(NEW_PASSWORD);
    await pageA.getByRole("button", { name: "変更" }).click();
    await expect(pageA.getByText(/パスワードを変更しました/)).toBeVisible();

    // Session A は維持
    await pageA.goto("/quiz");
    await expect(pageA).toHaveURL(/\/quiz/);

    // Session B は破棄され /login へ
    await pageB.goto("/quiz");
    await expect(pageB).toHaveURL(/\/login/);

    // 旧 PW でログイン不可
    await pageB.getByLabel(/ユーザー名/).fill(username);
    await pageB.getByLabel(/パスワード/).fill(PASSWORD);
    await pageB.getByRole("button", { name: "ログイン" }).click();
    await expect(pageB.getByText(/ユーザー名またはパスワードが違います/)).toBeVisible();

    // 新 PW でログイン可能
    await pageB.getByLabel(/ユーザー名/).fill(username);
    await pageB.getByLabel(/パスワード/).fill(NEW_PASSWORD);
    await pageB.getByRole("button", { name: "ログイン" }).click();
    await expect(pageB).toHaveURL(/\/quiz/);

    await ctxA.close();
    await ctxB.close();
  });
});
