# Readeck API Client (Dart)

A Dart client library for interacting with the [Readeck API](https://github.com/readeck/readeck). This client provides type-safe methods for accessing various API endpoints, leveraging the power of Freezed for data models and `http` for communication.

## Features

*   **Type-Safe API Calls:** Generated data models using Freezed ensure type safety and reduce runtime errors.
*   **Authentication:** Easy way to set and clear authentication tokens. Login method automatically stores the token.
*   **Implemented Endpoints:**
    *   Authentication (`/auth`, `/profile`)
    *   Bookmarks (CRUD, list, sync, article export, sharing)
    *   Labels (CRUD, list)
    *   Annotations (Highlights) (CRUD, list)
    *   Collections (CRUD, list)
    *   Imports (Text, Wallabag, Browser HTML, Pocket HTML)
*   **Error Handling:** Custom exceptions for different API error types.
*   **Multipart File Uploads:** Support for importing files via `multipart/form-data`.
*   **Non-JSON Response Handling:** Supports HTML and binary responses for specific endpoints (e.g., article content, EPUB export).

## Getting Started

### Prerequisites

*   Dart SDK installed.
*   A running Readeck instance.

### Installation

Add the following to your `pubspec.yaml` file:

```yaml
dependencies:
  http: ^1.0.0 # or any compatible version
  freezed_annotation: ^any # use appropriate version
  json_annotation: ^any # use appropriate version
  http_parser: ^0.8.0 # Recommended for MediaType when using multipart requests

  # If using this client from a local path:
  # readeck_api_client:
  #   path: path/to/readeck_client

  # If it gets published to pub.dev:
  # readeck_api_client: ^0.1.0
dev_dependencies:
  build_runner: ^any
  freezed: ^any
  json_serializable: ^any
```

Then run `dart pub get` or `flutter pub get`.

### Basic Usage

```dart
import 'dart:io'; // For File operations if reading from disk
import 'dart:typed_data'; // For Uint8List
import 'package:readeck_client/readeck_api_client.dart';
import 'package:readeck_client/models.dart';
// import 'package:http_parser/http_parser.dart'; // For MediaType if constructing it manually

void main() async {
  final apiClient = ReadeckApiClient(baseUrl: 'YOUR_READECK_BASE_URL');

  try {
    final authRequest = AuthRequest(
      username: 'your_username',
      password: 'your_password',
      application: 'MyDartApp',
    );
    // Login automatically sets the token in the client
    final authResponse = await apiClient.login(authRequest);
    print('Login successful! Token: ${authResponse.token}');

    final userProfile = await apiClient.getProfile();
    print('User Profile: ${userProfile.user?.username}');

    final bookmarks = await apiClient.listBookmarks(limit: 10);
    print('Fetched ${bookmarks.length} bookmarks.');
    for (var bookmark in bookmarks) {
      print('- ${bookmark.title} (${bookmark.url})');
    }

    // Example for file import (conceptual, replace with actual file reading)
    // Assuming you have Uint8List fileBytes and String filename:
    //
    // Example for browser bookmarks:
    // try {
    //   Uint8List browserFileBytes = await File('path/to/your/bookmarks.html').readAsBytes();
    //   String browserFilename = 'bookmarks.html';
    //   ApiMessageWithLocation importResult = await apiClient.importBrowserBookmarks(browserFileBytes, browserFilename);
    //   print('Browser bookmarks import initiated. Status: ${importResult.message.message}, Location: ${importResult.location}');
    // } on ApiException catch (e) {
    //   print('Browser bookmark import failed: ${e.message}');
    // }
    //
    // Example for Pocket bookmarks:
    // try {
    //   Uint8List pocketFileBytes = await File('path/to/your/pocket_export.html').readAsBytes();
    //   String pocketFilename = 'ril_export.html';
    //   ApiMessageWithLocation pocketImportResult = await apiClient.importPocketFile(pocketFileBytes, pocketFilename);
    //   print('Pocket bookmarks import initiated. Status: ${pocketImportResult.message.message}, Location: ${pocketImportResult.location}');
    // } on ApiException catch (e) {
    //   print('Pocket bookmark import failed: ${e.message}');
    // }

  } on UnauthorizedException catch (e) {
    print('Authentication failed: ${e.message}');
  } on ValidationException catch (e) {
    print('Validation error: ${e.message}');
    e.errors?.forEach((field, errors) {
      print('  $field: ${errors.join(', ')}');
    });
  } on ApiException catch (e) {
    print('An API error occurred: ${e.message}');
  } catch (e) {
    print('An unexpected error occurred: $e');
  } finally {
    apiClient.dispose();
  }
}
```

## Implemented Endpoints

So far, the following categories of endpoints are implemented:

*   **Authentication:**
    *   `POST /auth`: Login and get an API token. Token is auto-set in client.
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
*   **Annotations (Highlights):**
    *   `GET /bookmarks/annotations`: List all annotations for the current user.
    *   `GET /bookmarks/{bookmarkId}/annotations`: List annotations for a specific bookmark.
    *   `POST /bookmarks/{bookmarkId}/annotations`: Create an annotation.
    *   `PATCH /bookmarks/{bookmarkId}/annotations/{annotationId}`: Update an annotation.
    *   `DELETE /bookmarks/{bookmarkId}/annotations/{annotationId}`: Delete an annotation.
*   **Collections:**
    *   `GET /bookmarks/collections`: List all collections.
    *   `POST /bookmarks/collections`: Create a new collection.
    *   `GET /bookmarks/collections/{id}`: Get details of a specific collection.
    *   `PATCH /bookmarks/collections/{id}`: Update a collection.
    *   `DELETE /bookmarks/collections/{id}`: Delete a collection.
*   **Imports:**
    *   `POST /bookmarks/import/text`: Import bookmarks from a plain text file content.
    *   `POST /bookmarks/import/wallabag`: Import bookmarks from a Wallabag instance.
    *   `POST /bookmarks/import/browser`: Import browser bookmarks (HTML file via multipart/form-data).
    *   `POST /bookmarks/import/pocket-file`: Import Pocket export (HTML file via multipart/form-data).

## Future Work

*   Implement remaining API endpoints (if any, e.g., admin tools like `/cookbook/extract`).
*   Add comprehensive unit and integration tests.
*   Publish to pub.dev.
*   Refine error handling and model details based on further API testing.

## Contributing

Contributions are welcome! Please feel free to open an issue or submit a pull request.

---

*This client is currently under development.*
