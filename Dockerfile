# boki3-quizのビルドと実行イメージです。テンプレート・静的ファイル・マイグレーション・シードは
# すべてGoバイナリへ埋め込まれるため、実行イメージにはバイナリだけを置きます。
FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO不要 (modernc.org/sqlite) のため静的バイナリにします。
# TARGETARCHはBuildKitが自動設定します。手動buildでは未設定でも実行ホストと同archになります。
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-arm64} \
    go build -trimpath -ldflags="-s -w" -o /out/boki3-quiz ./cmd/boki3-quiz

# 実行イメージはシェルなしのdistrolessにします。nonrootユーザー (uid 65532) で動かします。
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/boki3-quiz /boki3-quiz

EXPOSE 8080

ENTRYPOINT ["/boki3-quiz"]
