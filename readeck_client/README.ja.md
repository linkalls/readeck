# Readeck API クライアント (Dart)

[Readeck API](https://github.com/readeck/readeck) と連携するためのDartクライアントライブラリです。このクライアントは、Freezedによるデータモデルと`http`パッケージによる通信を利用し、様々なAPIエンドポイントへ型安全にアクセスするメソッドを提供します。

## 特徴

*   **型安全なAPI呼び出し:** Freezedを使用して生成されたデータモデルにより、型安全性が保証され、実行時エラーを削減します。
*   **認証:** 認証トークンの設定とクリアが容易に行えます。
*   **実装済みエンドポイント (現時点):**
    *   認証 (`/auth`, `/profile`)
    *   ブックマーク (CRUD、一覧、同期、記事エクスポート、共有)
    *   ラベル (CRUD、一覧)
*   **エラーハンドリング:** APIのエラータイプに応じたカスタム例外を提供します。
*   **非JSONレスポンス対応:** 特定のエンドポイント（記事本文、EPUBエクスポートなど）におけるHTMLやバイナリレスポンスをサポートします。

## はじめに

### 必要条件

*   Dart SDK がインストールされていること。
*   Readeck のインスタンスが実行されていること。

### インストール

(公開後、またはpath/git依存として利用する場合) `pubspec.yaml` ファイルに以下を追加してください：

```yaml
dependencies:
  readeck_api_client: ^0.1.0 # 実際のバージョンまたは依存関係のタイプに置き換えてください
```

開発中の現在は、コードをローカルにお持ちであれば、ローカルパス依存として利用できます：

```yaml
dependencies:
  readeck_api_client:
    path: path/to/readeck_client # readeck_clientへのパス
```

その後、`dart pub get` または `flutter pub get` を実行してください。

### 基本的な使い方

```dart
import 'package:readeck_client/readeck_api_client.dart'; // 必要に応じてインポートパスを調整してください
// 特定のモデルをインポートする必要があるかもしれません:
// import 'package:readeck_client/models.dart'; // モデルのバレルファイルを想定

void main() async {
  // クライアントの初期化
  final apiClient = ReadeckApiClient(baseUrl: 'あなたのREADECK_BASE_URL');

  try {
    // 1. 認証 (ログイン)
    final authRequest = AuthRequest(
      username: 'your_username',
      password: 'your_password',
      application: 'MyDartApp',
    );
    final authResponse = await apiClient.login(authRequest);

    if (authResponse.token != null) {
      apiClient.setToken(authResponse.token!);
      print('ログイン成功! トークン: ${authResponse.token}');

      // 2. ユーザープロファイルの取得
      final userProfile = await apiClient.getProfile();
      print('ユーザープロファイル: ${userProfile.user?.username}');

      // 3. ブックマーク一覧の取得
      final bookmarks = await apiClient.listBookmarks(limit: 10);
      print('${bookmarks.length}件のブックマークを取得しました。');
      for (var bookmark in bookmarks) {
        print('- ${bookmark.title} (${bookmark.url})');
      }

      // 例: ブックマークの作成 (URLが有効であることを確認してください)
      // final newBookmark = await apiClient.createBookmark(
      //   BookmarkCreate(url: 'https_example.com_article', title: '新しいブックマーク')
      // );
      // print('作成されたブックマーク: ${newBookmark.title}');

    }
  } on UnauthorizedException catch (e) {
    print('認証に失敗しました: ${e.message}');
  } on ValidationException catch (e) {
    print('バリデーションエラー: ${e.message}');
    e.errors?.forEach((field, errors) {
      print('  $field: ${errors.join(', ')}');
    });
  } on ApiException catch (e) {
    print('APIエラーが発生しました: ${e.message}');
    print('ステータスコード: ${e.statusCode}');
    print('レスポンスボディ: ${e.responseBody}');
  } catch (e) {
    print('予期せぬエラーが発生しました: $e');
  } finally {
    apiClient.dispose(); // httpクライアントを閉じる
  }
}
```

## 実装済みエンドポイント

現在までに、以下のカテゴリのエンドポイントが実装されています：

*   **認証:**
    *   `POST /auth`: ログインし、APIトークンを取得します。
    *   `GET /profile`: 現在のユーザープロファイルを取得します。
*   **ブックマーク:**
    *   `GET /bookmarks`: フィルタリングとページネーション付きでブックマーク一覧を取得します。
    *   `GET /bookmarks/sync`: 同期のためにすべてのブックマークを一覧表示します。
    *   `POST /bookmarks`: 新しいブックマークを作成します。
    *   `GET /bookmarks/{id}`: 特定のブックマークの詳細を取得します。
    *   `PATCH /bookmarks/{id}`: ブックマークを更新します。
    *   `DELETE /bookmarks/{id}`: ブックマークを削除します。
    *   `GET /bookmarks/{id}/article`: 処理された記事のコンテンツ（HTML）を取得します。
    *   `GET /bookmarks/{id}/article.{format}`: ブックマークをエクスポートします（例: EPUB、Markdown）。
    *   `GET /bookmarks/{id}/share/link`: 公開共有可能なリンクを作成します。
    *   `POST /bookmarks/{id}/share/email`: ブックマークをメールで共有します。
*   **ラベル:**
    *   `GET /bookmarks/labels`: すべてのラベルを一覧表示します。
    *   `GET /bookmarks/labels/{name}`: 特定のラベルに関する情報を取得します。
    *   `PATCH /bookmarks/labels/{name}`: ラベル名を更新します。
    *   `DELETE /bookmarks/labels/{name}`: ラベルを削除します。

## 今後の作業

*   残りのAPIエンドポイントの実装:
    *   アノテーション（ハイライト）
    *   コレクション
    *   インポート（テキスト、Wallabagなど）
*   包括的なユニットテストと統合テストの追加。
*   pub.devへの公開。
*   さらなるAPIテストに基づくエラーハンドリングとモデル詳細の改良。

## 貢献

貢献を歓迎します！Issueを開いたり、プルリクエストを送信したりしてください。(標準的な貢献ガイドラインが適用されます)。

---

*このクライアントは現在開発中です。*
