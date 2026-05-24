import { test, expect, Page } from "@playwright/test";

const PASSWORD = "P@ssw0rd!Strong";

async function registerAndLogin(page: Page, username: string) {
  await page.goto("/register");
  await page.getByLabel(/ユーザー名/).fill(username);
  await page.getByLabel(/パスワード/).fill(PASSWORD);
  await page.getByRole("button", { name: "登録" }).click();
  await expect(page).toHaveURL(/\/quiz/);
}

test.describe("Quiz answering flow", () => {
  test("user answers a journal question, sees explanation, can navigate to next", async ({
    page,
  }) => {
    const username = `e2e_quiz_${Date.now().toString().slice(-6)}`;
    await registerAndLogin(page, username);

    // 学習画面: モードを Sequential に切り替えて 1 問目を確定的に取得する
    await page.locator("select[name='mode']").selectOption("sequential");
    await page.getByRole("button", { name: "切替" }).click();
    await expect(page).toHaveURL(/\/quiz/);

    const prompt = await page.locator(".prompt").innerText();
    expect(prompt.length).toBeGreaterThan(5);

    // 仕訳問題の最も典型的なパターン (現金 / 売上) を埋める。
    // ジェネレータの cash-sale-001 は「商品 1,000 円 を売り上げ、代金を現金で受け取った」
    await page.locator("input[name='debit_account_1']").fill("現金");
    await page.locator("input[name='debit_amount_1']").fill("1000");
    await page.locator("input[name='credit_account_1']").fill("売上");
    await page.locator("input[name='credit_amount_1']").fill("1000");
    await page.getByRole("button", { name: "採点" }).click();

    // 採点結果ページ
    await expect(page.locator(".result")).toBeVisible();
    await expect(page.getByRole("link", { name: "次の問題へ" })).toBeVisible();
    await expect(page.getByRole("link", { name: "進捗を見る" })).toBeVisible();
  });

  test("incorrect answer shows 不正解 result", async ({ page }) => {
    const username = `e2e_quiz_ng_${Date.now().toString().slice(-6)}`;
    await registerAndLogin(page, username);
    await page.locator("select[name='mode']").selectOption("sequential");
    await page.getByRole("button", { name: "切替" }).click();

    await page.locator("input[name='debit_account_1']").fill("間違い");
    await page.locator("input[name='debit_amount_1']").fill("999");
    await page.locator("input[name='credit_account_1']").fill("売上");
    await page.locator("input[name='credit_amount_1']").fill("999");
    await page.getByRole("button", { name: "採点" }).click();

    await expect(page.locator(".result.ng")).toContainText("不正解");
  });
});
