import { test, expect, request as pwRequest } from "@playwright/test";

const PASSWORD = "P@ssw0rd!Strong";

test.describe("API /api/v1/* (JWT bearer)", () => {
  test("login -> next -> answer -> summary -> history -> delete", async ({ baseURL, page }) => {
    const username = `e2e_api_${Date.now().toString().slice(-6)}`;
    // ユーザ作成は HTML 経路で
    await page.goto("/register");
    await page.getByLabel(/ユーザー名/).fill(username);
    await page.getByLabel(/パスワード/).fill(PASSWORD);
    await page.getByRole("button", { name: "登録" }).click();
    await expect(page).toHaveURL(/\/quiz/);

    const api = await pwRequest.newContext({ baseURL });

    // API ログイン
    const loginR = await api.post("/api/v1/auth/login", {
      data: { username, password: PASSWORD },
      headers: { "Content-Type": "application/json" },
    });
    expect(loginR.status()).toBe(200);
    const loginBody = await loginR.json();
    const token = loginBody.token;
    expect(token).toBeTruthy();
    expect(new Date(loginBody.expires_at).getTime()).toBeGreaterThan(Date.now());

    const authHeader = { Authorization: `Bearer ${token}` };

    // 次の問題 (Go の encoding/json は field 名のまま出力するので ID は大文字)
    const nextR = await api.get("/api/v1/quiz/next?set=core_300&mode=sequential", {
      headers: authHeader,
    });
    expect(nextR.status()).toBe(200);
    const nextBody = await nextR.json();
    const question = nextBody.question;
    expect(question.ID).toBeGreaterThan(0);

    // 解答 (cash-sale-001 が確実に出題される sequential mode)
    const answerR = await api.post("/api/v1/quiz/answer", {
      data: {
        question_id: question.ID,
        set_code: "core_300",
        duration_ms: 4000,
        answer: {
          Type: "journal",
          Debits: [{ Account: "現金", Amount: 1000 }],
          Credits: [{ Account: "売上", Amount: 1000 }],
        },
      },
      headers: { ...authHeader, "Content-Type": "application/json" },
    });
    expect(answerR.status()).toBe(200);

    // summary
    const sumR = await api.get("/api/v1/stats/summary", { headers: authHeader });
    expect(sumR.status()).toBe(200);
    const summary = await sumR.json();
    expect(summary.TotalAttempts).toBeGreaterThanOrEqual(1);

    // history
    const histR = await api.get("/api/v1/history?limit=10", { headers: authHeader });
    expect(histR.status()).toBe(200);
    const { attempts } = await histR.json();
    expect(attempts.length).toBeGreaterThanOrEqual(1);

    // delete
    const delR = await api.delete(`/api/v1/history/${attempts[0].ID}`, { headers: authHeader });
    expect(delR.status()).toBe(204);

    await api.dispose();
  });

  test("invalid bearer returns 401 with json body", async ({ baseURL }) => {
    const api = await pwRequest.newContext({ baseURL });
    const r = await api.get("/api/v1/stats/summary", {
      headers: { Authorization: "Bearer not-a-real-token" },
    });
    expect(r.status()).toBe(401);
    expect(r.headers()["content-type"]).toContain("application/json");
    await api.dispose();
  });

  test("missing bearer returns 401", async ({ baseURL }) => {
    const api = await pwRequest.newContext({ baseURL });
    const r = await api.get("/api/v1/stats/summary");
    expect(r.status()).toBe(401);
    await api.dispose();
  });
});
