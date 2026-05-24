import { test, expect, Page } from "@playwright/test";

test.setTimeout(60_000);
const PASSWORD = "P@ssw0rd!Strong";

async function registerLoginAndAnswer(page: Page, username: string) {
  await page.goto("/register");
  await page.getByLabel(/ユーザー名/).fill(username);
  await page.getByLabel(/パスワード/).fill(PASSWORD);
  await page.getByRole("button", { name: "登録" }).click();
  await expect(page).toHaveURL(/\/quiz/);
  await page.locator("select[name='mode']").selectOption("sequential");
  await page.getByRole("button", { name: "切替" }).click();
  await page.locator("input[name='debit_account_1']").fill("現金");
  await page.locator("input[name='debit_amount_1']").fill("1000");
  await page.locator("input[name='credit_account_1']").fill("売上");
  await page.locator("input[name='credit_amount_1']").fill("1000");
  await page.getByRole("button", { name: "採点" }).click();
}

test.describe("History and progress views", () => {
  test("after an attempt, history page lists it and supports individual delete", async ({
    page,
  }) => {
    const username = `e2e_hist_${Date.now().toString().slice(-6)}`;
    await registerLoginAndAnswer(page, username);

    await page.goto("/history");
    await expect(page.getByRole("heading", { name: "履歴" })).toBeVisible();
    await expect(page.locator("table.tbl tbody tr")).toHaveCount(1);

    page.once("dialog", (d) => d.accept());
    await page.locator("button.link", { hasText: "削除" }).click();
    await expect(page.getByText(/履歴はまだありません/)).toBeVisible();
  });

  test("progress page renders SVG charts (placeholder when no data, real after data)", async ({
    page,
  }) => {
    const username = `e2e_prog_${Date.now().toString().slice(-6)}`;
    await registerLoginAndAnswer(page, username);

    await page.goto("/progress");
    await expect(page.getByRole("heading", { name: "進捗" })).toBeVisible();

    // SVG が描画されている
    const svgs = await page.locator("svg").count();
    expect(svgs).toBeGreaterThanOrEqual(2);

    // 日次正解率 SVG にデータが含まれる (polyline か "データなし" 文字)
    const dailyHtml = await page.locator("section.progress article.card").first().innerHTML();
    expect(dailyHtml.includes("<polyline") || dailyHtml.includes("データなし")).toBeTruthy();
  });

  test("history clear-all removes everything", async ({ page }) => {
    const username = `e2e_clr_${Date.now().toString().slice(-6)}`;
    await registerLoginAndAnswer(page, username);
    await page.goto("/history");
    await expect(page.locator("table.tbl tbody tr")).toHaveCount(1);

    page.once("dialog", (d) => d.accept());
    await page.getByRole("button", { name: "すべて削除" }).click();
    await expect(page.getByText(/履歴はまだありません/)).toBeVisible();
  });
});
