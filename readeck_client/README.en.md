# Readeck API Client (Dart)

A Dart client library for interacting with the [Readeck API](https://github.com/readeck/readeck). This client provides type-safe methods for accessing various API endpoints, leveraging the power of Freezed for data models and `http` for communication.

## Features

*   **Type-Safe API Calls:** Generated data models using Freezed ensure type safety and reduce runtime errors.
*   **Authentication:** Easy way to set and clear authentication tokens.
*   **Implemented Endpoints (so far):**
    *   Authentication (`/auth`, `/profile`)
    *   Bookmarks (CRUD, list, sync, article export, sharing)
    *   Labels (CRUD, list)
*   **Error Handling:** Custom exceptions for different API error types.
*   **Non-JSON Response Handling:** Supports HTML and binary responses for specific endpoints (e.g., article content, EPUB export).

## Getting Started

### Prerequisites

*   Dart SDK installed.
*   A running Readeck instance.

### Installation

Add the following to your `pubspec.yaml` file (once published or if using as a path/git dependency):

```yaml
dependencies:
  readeck_api_client: ^0.1.0 # Replace with actual version or dependency type
```

For now, as it's being developed, you might use it as a local path dependency if you have the code locally:

```yaml
dependencies:
  readeck_api_client:
    path: path/to/readeck_client
```

Then run `dart pub get` or `flutter pub get`.

### Basic Usage

```dart
import 'package:readeck_client/readeck_api_client.dart'; // Adjust import path as needed
// You might also need to import specific models:
// import 'package:readeck_client/models.dart'; // Assuming a barrel file for models

void main() async {
  // Initialize the client
  final apiClient = ReadeckApiClient(baseUrl: 'YOUR_READECK_BASE_URL');

  try {
    // 1. Authenticate (Login)
    final authRequest = AuthRequest(
      username: 'your_username',
      password: 'your_password',
      application: 'MyDartApp',
    );
    final authResponse = await apiClient.login(authRequest);

    if (authResponse.token != null) {
      apiClient.setToken(authResponse.token!);
      print('Login successful! Token: ${authResponse.token}');

      // 2. Get User Profile
      final userProfile = await apiClient.getProfile();
      print('User Profile: ${userProfile.user?.username}');

      // 3. List Bookmarks
      final bookmarks = await apiClient.listBookmarks(limit: 10);
      print('Fetched ${bookmarks.length} bookmarks.');
      for (var bookmark in bookmarks) {
        print('- ${bookmark.title} (${bookmark.url})');
      }

      // Example: Create a bookmark (ensure URL is valid)
      // final newBookmark = await apiClient.createBookmark(
      //   BookmarkCreate(url: 'https_example.com_article', title: 'My New Bookmark')
      // );
      // print('Created bookmark: ${newBookmark.title}');

    }
  } on UnauthorizedException catch (e) {
    print('Authentication failed: ${e.message}');
  } on ValidationException catch (e) {
    print('Validation error: ${e.message}');
    e.errors?.forEach((field, errors) {
      print('  $field: ${errors.join(', ')}');
    });
  } on ApiException catch (e) {
    print('An API error occurred: ${e.message}');
    print('Status Code: ${e.statusCode}');
    print('Response Body: ${e.responseBody}');
  } catch (e) {
    print('An unexpected error occurred: $e');
  } finally {
    apiClient.dispose(); // Close the http client
  }
}
```

## Implemented Endpoints

So far, the following categories of endpoints are implemented:

*   **Authentication:**
    *   `POST /auth`: Login and get an API token.
    *   `GET /profile`: Get the current user's profile.
*   **Bookmarks:**
    *   `GET /bookmarks`: List bookmarks with filtering and pagination.
    *   `GET /bookmarks/sync`: List all bookmarks for synchronization.
    *   `POST /bookmarks`: Create a new bookmark.
    *   `GET /bookmarks/{id}`: Get details of a specific bookmark.
    *   `PATCH /bookmarks/{id}`: Update a bookmark.
    *   `DELETE /bookmarks/{id}`: Delete a bookmark.
    *   `GET /bookmarks/{id}/article`: Get the processed article content (HTML).
    *   `GET /bookmarks/{id}/article.{format}`: Export bookmark (e.g., EPUB, Markdown).
    *   `GET /bookmarks/{id}/share/link`: Create a public shareable link.
    *   `POST /bookmarks/{id}/share/email`: Share a bookmark via email.
*   **Labels:**
    *   `GET /bookmarks/labels`: List all labels.
    *   `GET /bookmarks/labels/{name}`: Get information about a specific label.
    *   `PATCH /bookmarks/labels/{name}`: Update a label's name.
    *   `DELETE /bookmarks/labels/{name}`: Delete a label.

## Future Work

*   Implement remaining API endpoints:
    *   Annotations (Highlights)
    *   Collections
    *   Imports (Text, Wallabag, etc.)
*   Add comprehensive unit and integration tests.
*   Publish to pub.dev.
*   Refine error handling and model details based on further API testing.

## Contributing

Contributions are welcome! Please feel free to open an issue or submit a pull request. (Standard contribution guidelines would apply).

---

*This client is currently under development.*
