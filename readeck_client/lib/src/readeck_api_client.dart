import 'dart:convert';
import 'dart:io'; // For HttpHeaders
import 'dart:typed_data'; // For Uint8List

import 'package:http/http.dart' as http;
// Required for MultipartRequest and MultipartFile
import 'package:http_parser/http_parser.dart' show MediaType;

// Models are now imported via the barrel file
import 'models/models.dart';
import 'exceptions.dart';

/// A Dart client for interacting with the Readeck API.
///
/// Provides methods to access various Readeck functionalities including
/// authentication, bookmark management, label organization, annotations,
/// collections, and data import.
class ReadeckApiClient {
  /// The base URL of the Readeck API instance.
  final String baseUrl;
  String? _token;
  http.Client _httpClient;

  /// Creates a new instance of [ReadeckApiClient].
  ///
  /// [baseUrl] is the base URL of the Readeck API.
  /// [token] is an optional initial authentication token.
  /// [httpClient] is an optional HTTP client for testing purposes.
  ReadeckApiClient({required this.baseUrl, String? token, http.Client? httpClient})
      : _token = token,
        _httpClient = httpClient ?? http.Client();

  /// Sets the authentication token for subsequent API requests.
  void setToken(String token) {
    _token = token;
  }

  /// Clears the authentication token.
  void clearToken() {
    _token = null;
  }

  /// Prepares the HTTP headers for an API request.
  ///
  /// Includes common headers like Content-Type and Accept.
  /// If an authentication token is set, it adds the Authorization header.
  /// [contentType] defaults to 'application/json; charset=utf-8'. Can be null for multipart.
  /// [accept] defaults to 'application/json'.
  Map<String, String> _getHeaders({String? contentType = 'application/json; charset=utf-8', String accept = 'application/json'}) {
    final headers = <String, String>{
      if (contentType != null) HttpHeaders.contentTypeHeader: contentType, // Only add if specified
      HttpHeaders.acceptHeader: accept,
    };
    if (_token != null) {
      headers[HttpHeaders.authorizationHeader] = 'Bearer $_token';
    }
    return headers;
  }

  /// Decodes the HTTP response body.
  ///
  /// If [expectJson] is true, attempts to decode as JSON. If decoding fails,
  /// an [ApiException] is thrown as the expectation was not met.
  /// Otherwise, decodes based on Content-Type header (HTML, Markdown as string, EPUB as Uint8List)
  /// or defaults to UTF-8 string.
  dynamic _decodeResponse(http.Response response, {bool expectJson = true}) {
    if (response.bodyBytes.isEmpty) {
      return null;
    }
    if (expectJson) {
      try {
        return jsonDecode(utf8.decode(response.bodyBytes));
      } catch (e) {
        // If JSON was expected and parsing failed, it's an issue.
        throw ApiException(
            'Expected JSON response but failed to decode. Body: ${utf8.decode(response.bodyBytes)}',
            statusCode: response.statusCode,
            responseBody: utf8.decode(response.bodyBytes)
        );
      }
    }
    // Handling for non-JSON expected responses
    final contentTypeHeader = response.headers[HttpHeaders.contentTypeHeader];
    if (contentTypeHeader != null) {
      if (contentTypeHeader.contains('text/html') || contentTypeHeader.contains('text/markdown')) {
          return utf8.decode(response.bodyBytes);
      } else if (contentTypeHeader.contains('application/epub+zip')) {
          return response.bodyBytes; // Return as Uint8List
      }
    }
    // Default for non-JSON and unspecified/unhandled content type
    return utf8.decode(response.bodyBytes);
  }

  /// Handles the HTTP response, checking for errors and decoding the body.
  ///
  /// Throws an appropriate [ApiException] or its subclass if an error occurs.
  /// Returns the decoded body on success.
  /// [expectJson] indicates if the successful response body is expected to be JSON.
  Future<dynamic> _handleResponse(http.Response response, {bool expectJson = true}) async {
    // Handle successful non-JSON responses first
    if (response.statusCode >= 200 && response.statusCode < 300 && !expectJson) {
        return _decodeResponse(response, expectJson: false);
    }

    dynamic decodedBody;
    // Errors (4xx, 5xx) and expected JSON responses are decoded as JSON.
    // If expectJson is true, or if it's an error status, attempt to decode as JSON.
    if (expectJson || (response.statusCode >= 400 && response.statusCode < 600)) {
        decodedBody = _decodeResponse(response, expectJson: true);
    } else {
        decodedBody = _decodeResponse(response, expectJson: false);
    }

    if (response.statusCode >= 200 && response.statusCode < 300) {
      return decodedBody;
    }
    // Error handling from here (decodedBody here is from expectJson=true attempt for errors)
    else if (response.statusCode == 401) {
      throw UnauthorizedException(
        decodedBody is Map ? decodedBody['message'] ?? 'Unauthorized' : decodedBody.toString(),
        responseBody: decodedBody
      );
    } else if (response.statusCode == 403) {
      throw ForbiddenException(
        decodedBody is Map ? decodedBody['message'] ?? 'Forbidden' : decodedBody.toString(),
        responseBody: decodedBody
      );
    } else if (response.statusCode == 404) {
      throw NotFoundException(
        decodedBody is Map ? decodedBody['message'] ?? 'Not Found' : decodedBody.toString(),
        responseBody: decodedBody
      );
    } else if (response.statusCode == 422) {
      final apiError = decodedBody is Map && decodedBody.containsKey('isValid')
                       ? ApiError.fromJson(decodedBody.cast<String,dynamic>())
                       : null;
      throw ValidationException(
        apiError?.message ?? (decodedBody is Map ? decodedBody['message'] ?? 'Validation Error' : decodedBody.toString()),
        errors: apiError?.fields?.map((key, value) => MapEntry(key, value.errors ?? [])),
        responseBody: decodedBody
      );
    } else if (response.statusCode >= 500 && response.statusCode < 600) {
      throw InternalServerErrorException(
        decodedBody is Map ? decodedBody['message'] ?? 'Internal Server Error' : decodedBody.toString(),
        responseBody: decodedBody
      );
    }
     else {
      throw ApiException(
        decodedBody is Map ? decodedBody['message'] ?? 'API Error' : decodedBody.toString(),
        statusCode: response.statusCode,
        responseBody: decodedBody
      );
    }
  }

  /// Generic helper method to make an API request.
  ///
  /// [requestFunction] is a function that returns a `Future<http.Response>`.
  /// [fromBody] is a function that converts the decoded response body to type [T].
  /// [expectJsonResponse] indicates if the successful response body should be JSON.
  Future<T> _makeRequest<T>(
    Future<http.Response> Function() requestFunction,
    T Function(dynamic body) fromBody, {bool expectJsonResponse = true}
  ) async {
    final response = await requestFunction();
    final dynamic body = await _handleResponse(response, expectJson: expectJsonResponse);
    if (body == null && null is! T && expectJsonResponse) {
        throw ApiException('Expected non-null JSON response for ${T.toString()}, but received null body', statusCode: response.statusCode);
    }
    return fromBody(body);
  }

  /// Helper method for requests that need the raw `http.Response` object,
  /// typically to access headers (e.g., for 202 Location). It still performs error checking.
  Future<http.Response> _makeRawRequest(
    Future<http.Response> Function() requestFunction,
  ) async {
    final response = await requestFunction();
    if (response.statusCode >= 400) {
        await _handleResponse(response, expectJson: true);
    }
    return response;
  }

  /// Helper method for requests that do not return a meaningful body on success (e.g., 204 No Content).
  Future<void> _makeRequestVoidReturn(
    Future<http.Response> Function() requestFunction,
  ) async {
    final response = await requestFunction();
    await _handleResponse(response, expectJson: response.statusCode >= 400);
  }

  /// Helper for multipart file uploads.
  Future<http.Response> _makeMultipartRequest(
    String path,
    List<int> fileBytes,
    String fieldName,
    String filename,
    {MediaType? contentType}
  ) async {
    final uri = Uri.parse('$baseUrl$path'); // Corrected: Ensure path starts with / if it's a segment
    final request = http.MultipartRequest('POST', uri);

    final commonHeaders = _getHeaders(contentType: null);
    request.headers.addAll(commonHeaders);

    final multipartFile = http.MultipartFile.fromBytes(
      fieldName,
      fileBytes,
      filename: filename,
      contentType: contentType,
    );
    request.files.add(multipartFile);

    final streamedResponse = await _httpClient.send(request);
    return http.Response.fromStream(streamedResponse);
  }


  /// Builds a map of query parameters for HTTP requests.
  /// Null values are removed, except for the 'labels' key where null is converted to an empty string.
  /// List values are joined by commas.
  Map<String, String> _buildQueryParameters(Map<String, dynamic> params) {
    return params.entries
        .where((entry) {
          if (entry.key == 'labels' && entry.value == null) return true;
          return entry.value != null;
        })
        .map((entry) {
          if (entry.value is List) {
            return MapEntry(entry.key, (entry.value as List).join(','));
          }
          return MapEntry(entry.key, entry.value?.toString() ?? '');
        })
        .fold<Map<String, String>>({}, (map, entry) {
          if (entry.value.isNotEmpty || entry.key == 'labels') {
             map[entry.key] = entry.value;
          }
          return map;
        });
  }

  // --- Auth Endpoints ---

  /// Authenticates the user and retrieves an API token.
  /// If successful, the token is automatically stored in the client instance.
  /// Corresponds to `POST /auth`.
  Future<AuthResponse> login(AuthRequest authRequest) async {
    final authResponse = await _makeRequest(
      () => _httpClient.post(
        Uri.parse('$baseUrl/auth'),
        headers: _getHeaders(),
        body: jsonEncode(authRequest.toJson()),
      ),
      (json) => AuthResponse.fromJson(json as Map<String, dynamic>),
    );

    if (authResponse.token != null) {
      setToken(authResponse.token!);
    }
    return authResponse;
  }

  /// Retrieves the current user's profile information.
  /// Corresponds to `GET /profile`.
  Future<UserProfile> getProfile() async {
    return _makeRequest(
      () => _httpClient.get(
        Uri.parse('$baseUrl/profile'),
        headers: _getHeaders(),
      ),
      (json) => UserProfile.fromJson(json as Map<String, dynamic>),
    );
  }

  /// Logs out the current user by invalidating the session/token on the server.
  /// Corresponds to `DELETE /auth`.
  Future<void> logout() async {
    await _makeRequestVoidReturn(
      () => _httpClient.delete(
        Uri.parse('$baseUrl/auth'),
        headers: _getHeaders(),
      ),
    );
  }

  // --- Bookmark Endpoints ---
  Future<List<BookmarkSummary>> listBookmarks({
    String? search, String? title, String? author, String? site,
    List<String>? type, String? labels, bool? isLoaded, bool? hasErrors,
    bool? hasLabels, bool? isMarked, bool? isArchived, String? rangeStart,
    String? rangeEnd, List<String>? readStatus, String? id, String? collection,
    int? limit, int? offset,
  }) async {
    final queryParams = _buildQueryParameters({
      'search': search, 'title': title, 'author': author, 'site': site,
      'type': type, 'labels': labels, 'is_loaded': isLoaded, 'has_errors': hasErrors,
      'has_labels': hasLabels, 'is_marked': isMarked, 'is_archived': isArchived,
      'range_start': rangeStart, 'range_end': rangeEnd, 'read_status': readStatus,
      'id': id, 'collection': collection, 'limit': limit, 'offset': offset,
    });
    return _makeRequest(
      () => _httpClient.get(
        Uri.parse('$baseUrl/bookmarks').replace(queryParameters: queryParams.isEmpty ? null : queryParams),
        headers: _getHeaders(),
      ),
      (json) => (json as List)
          .map((item) => BookmarkSummary.fromJson(item as Map<String, dynamic>))
          .toList()
    );
  }

  Future<List<BookmarkSync>> syncBookmarks() async {
    return _makeRequest(
      () => _httpClient.get(
        Uri.parse('$baseUrl/bookmarks/sync'),
        headers: _getHeaders(),
      ),
      (json) => (json as List)
          .map((item) => BookmarkSync.fromJson(item as Map<String, dynamic>))
          .toList()
    );
  }

  Future<BookmarkInfo> createBookmark(BookmarkCreate bookmarkCreate) async {
    final response = await _makeRawRequest(
      () => _httpClient.post(
        Uri.parse('$baseUrl/bookmarks'),
        headers: _getHeaders(),
        body: jsonEncode(bookmarkCreate.toJson()),
      ),
    );

    if (response.statusCode == 202) {
        final bookmarkId = response.headers['bookmark-id'];
        if (bookmarkId != null) {
            return getBookmark(bookmarkId);
        } else {
            final dynamic body = _decodeResponse(response, expectJson: true);
            throw ApiException(
                "Bookmark creation initiated (202), but 'bookmark-id' header was missing.",
                statusCode: response.statusCode,
                responseBody: body);
        }
    }
    final dynamic body = _decodeResponse(response, expectJson: true);
    if (body is Map<String, dynamic>) {
        return BookmarkInfo.fromJson(body);
    }
    throw ApiException(
        "Unexpected response after creating bookmark. Status: ${response.statusCode}, Body: $body",
        statusCode: response.statusCode,
        responseBody: body);
  }

  Future<BookmarkInfo> getBookmark(String id) async {
    return _makeRequest(
      () => _httpClient.get(
        Uri.parse('$baseUrl/bookmarks/$id'),
        headers: _getHeaders(),
      ),
      (json) => BookmarkInfo.fromJson(json as Map<String, dynamic>),
    );
  }

  Future<BookmarkUpdated> updateBookmark(String id, BookmarkUpdate bookmarkUpdate) async {
    return _makeRequest(
      () => _httpClient.patch(
        Uri.parse('$baseUrl/bookmarks/$id'),
        headers: _getHeaders(),
        body: jsonEncode(bookmarkUpdate.toJson()),
      ),
      (json) => BookmarkUpdated.fromJson(json as Map<String, dynamic>),
    );
  }

  Future<void> deleteBookmark(String id) async {
    await _makeRequestVoidReturn(
      () => _httpClient.delete(
        Uri.parse('$baseUrl/bookmarks/$id'),
        headers: _getHeaders(),
      ),
    );
  }

  Future<String> getBookmarkArticle(String id) async {
    return _makeRequest(
      () => _httpClient.get(
        Uri.parse('$baseUrl/bookmarks/$id/article'),
        headers: _getHeaders(accept: 'text/html; charset=utf-8'),
      ),
      (body) => body as String,
      expectJsonResponse: false,
    );
  }

  Future<dynamic> exportBookmark(String id, String format) async {
    String acceptHeader;
    if (format == 'epub') {
      acceptHeader = 'application/epub+zip';
    } else if (format == 'md') {
      acceptHeader = 'text/markdown; charset=utf-8';
    } else {
      throw ArgumentError('Unsupported format: $format. Must be "epub" or "md".');
    }

    return _makeRequest(
      () => _httpClient.get(
        Uri.parse('$baseUrl/bookmarks/$id/article.$format'),
        headers: _getHeaders(accept: acceptHeader),
      ),
      (body) {
        if (format == 'epub' && body is Uint8List) return body;
        if (format == 'md' && body is String) return body;
        throw ApiException(
            "Received unexpected body type for format '$format'. Expected ${format == 'epub' ? 'Uint8List' : 'String'}, got ${body.runtimeType}",
            responseBody: body is Uint8List ? "Binary data (Uint8List)" : body.toString()
        );
      },
      expectJsonResponse: false,
    );
  }

  Future<BookmarkShareLink> shareBookmarkLink(String id) async {
    return _makeRequest(
      () => _httpClient.get(
        Uri.parse('$baseUrl/bookmarks/$id/share/link'),
        headers: _getHeaders(),
      ),
      (json) => BookmarkShareLink.fromJson(json as Map<String, dynamic>),
    );
  }

  Future<ApiMessage> shareBookmarkEmail(String id, BookmarkShareEmail shareEmailData) async {
    return _makeRequest(
      () => _httpClient.post(
        Uri.parse('$baseUrl/bookmarks/$id/share/email'),
        headers: _getHeaders(),
        body: jsonEncode(shareEmailData.toJson()),
      ),
      (json) => ApiMessage.fromJson(json as Map<String, dynamic>),
    );
  }

  // --- Label Endpoints ---
  Future<List<LabelInfo>> listLabels() async {
    return _makeRequest(
      () => _httpClient.get(
        Uri.parse('$baseUrl/bookmarks/labels'),
        headers: _getHeaders(),
      ),
      (json) => (json as List)
          .map((item) => LabelInfo.fromJson(item as Map<String, dynamic>))
          .toList()
    );
  }

  Future<List<LabelInfo>> getLabelInfo(String name) async {
    return _makeRequest(
      () => _httpClient.get(
        Uri.parse('$baseUrl/bookmarks/labels/${Uri.encodeComponent(name)}'),
        headers: _getHeaders(),
      ),
      (json) => (json as List)
          .map((item) => LabelInfo.fromJson(item as Map<String, dynamic>))
          .toList()
    );
  }

  Future<ApiMessage> updateLabel(String name, LabelUpdate labelUpdate) async {
    return _makeRequest(
      () => _httpClient.patch(
        Uri.parse('$baseUrl/bookmarks/labels/${Uri.encodeComponent(name)}'),
        headers: _getHeaders(),
        body: jsonEncode(labelUpdate.toJson()),
      ),
      (json) => ApiMessage.fromJson(json as Map<String, dynamic>),
    );
  }

  Future<void> deleteLabel(String name) async {
    await _makeRequestVoidReturn(
      () => _httpClient.delete(
        Uri.parse('$baseUrl/bookmarks/labels/${Uri.encodeComponent(name)}'),
        headers: _getHeaders(),
      ),
    );
  }

  // --- Annotation (Highlight) Endpoints ---
  Future<List<AnnotationSummary>> listAnnotations({int? limit, int? offset}) async {
    final queryParams = _buildQueryParameters({'limit': limit, 'offset': offset});
    return _makeRequest(
      () => _httpClient.get(
        Uri.parse('$baseUrl/bookmarks/annotations').replace(queryParameters: queryParams.isEmpty ? null : queryParams),
        headers: _getHeaders(),
      ),
      (json) => (json as List)
          .map((item) => AnnotationSummary.fromJson(item as Map<String, dynamic>))
          .toList()
    );
  }

  Future<List<AnnotationInfo>> listBookmarkAnnotations(String bookmarkId) async {
    return _makeRequest(
      () => _httpClient.get(
        Uri.parse('$baseUrl/bookmarks/$bookmarkId/annotations'),
        headers: _getHeaders(),
      ),
      (json) => (json as List)
          .map((item) => AnnotationInfo.fromJson(item as Map<String, dynamic>))
          .toList()
    );
  }

  Future<AnnotationInfo> createBookmarkAnnotation(String bookmarkId, AnnotationCreate annotationCreate) async {
    return _makeRequest(
      () => _httpClient.post(
        Uri.parse('$baseUrl/bookmarks/$bookmarkId/annotations'),
        headers: _getHeaders(),
        body: jsonEncode(annotationCreate.toJson()),
      ),
      (json) => AnnotationInfo.fromJson(json as Map<String, dynamic>),
    );
  }

  Future<AnnotationUpdateResponse> updateBookmarkAnnotation(String bookmarkId, String annotationId, AnnotationUpdate annotationUpdate) async {
    return _makeRequest(
      () => _httpClient.patch(
        Uri.parse('$baseUrl/bookmarks/$bookmarkId/annotations/$annotationId'),
        headers: _getHeaders(),
        body: jsonEncode(annotationUpdate.toJson()),
      ),
      (json) => AnnotationUpdateResponse.fromJson(json as Map<String, dynamic>),
    );
  }

  Future<void> deleteBookmarkAnnotation(String bookmarkId, String annotationId) async {
    await _makeRequestVoidReturn(
      () => _httpClient.delete(
        Uri.parse('$baseUrl/bookmarks/$bookmarkId/annotations/$annotationId'),
        headers: _getHeaders(),
      ),
    );
  }

  // --- Collection Endpoints ---
  Future<List<CollectionInfo>> listCollections({int? limit, int? offset}) async {
    final queryParams = _buildQueryParameters({'limit': limit, 'offset': offset});
    return _makeRequest(
      () => _httpClient.get(
        Uri.parse('$baseUrl/bookmarks/collections').replace(queryParameters: queryParams.isEmpty ? null : queryParams),
        headers: _getHeaders(),
      ),
      (json) => (json as List)
          .map((item) => CollectionInfo.fromJson(item as Map<String, dynamic>))
          .toList()
    );
  }

  Future<CollectionInfo> createCollection(CollectionCreateOrUpdate collectionCreate) async {
    return _makeRequest(
      () => _httpClient.post(
        Uri.parse('$baseUrl/bookmarks/collections'),
        headers: _getHeaders(),
        body: jsonEncode(collectionCreate.toJson()),
      ),
      (json) => CollectionInfo.fromJson(json as Map<String, dynamic>),
    );
  }

  Future<CollectionInfo> getCollectionInfo(String id) async {
    return _makeRequest(
      () => _httpClient.get(
        Uri.parse('$baseUrl/bookmarks/collections/$id'),
        headers: _getHeaders(),
      ),
      (json) => CollectionInfo.fromJson(json as Map<String, dynamic>),
    );
  }

  Future<CollectionSummary> updateCollection(String id, CollectionCreateOrUpdate collectionUpdate) async {
    return _makeRequest(
      () => _httpClient.patch(
        Uri.parse('$baseUrl/bookmarks/collections/$id'),
        headers: _getHeaders(),
        body: jsonEncode(collectionUpdate.toJson()),
      ),
      (json) => CollectionSummary.fromJson(json as Map<String, dynamic>),
    );
  }

  Future<void> deleteCollection(String id) async {
    await _makeRequestVoidReturn(
      () => _httpClient.delete(
        Uri.parse('$baseUrl/bookmarks/collections/$id'),
        headers: _getHeaders(),
      ),
    );
  }

  // --- Import Endpoints ---
  Future<ApiMessageWithLocation> _importMultipartFile(
      String pathSegment, List<int> fileBytes, String filename, {MediaType? fileContentType}) async {
    final response = await _makeMultipartRequest(
      '/bookmarks/import/$pathSegment', // Path segment for specific import type
      fileBytes,
      'data', // field name expected by the server for the file
      filename,
      contentType: fileContentType,
    );
    // Assuming 202 Accepted with Location header and ApiMessage body
    final dynamic jsonBody = _decodeResponse(response, expectJson: true);
    final location = response.headers['location'];
    if (location == null) {
      throw ApiException("Import task accepted (202) but Location header missing.", statusCode: response.statusCode, responseBody: jsonBody);
    }
    return ApiMessageWithLocation(
      message: ApiMessage.fromJson(jsonBody as Map<String, dynamic>),
      location: location,
    );
  }

  /// Imports bookmarks from a browser export file (HTML).
  /// [fileBytes] are the raw bytes of the HTML file.
  /// [filename] is the name of the file (e.g., "bookmarks.html").
  /// Returns an [ApiMessageWithLocation] containing the API message and the Location header of the import task.
  Future<ApiMessageWithLocation> importBrowserBookmarks(List<int> fileBytes, String filename) async {
    return _importMultipartFile('browser', fileBytes, filename, fileContentType: MediaType('text', 'html'));
  }

  /// Imports bookmarks from a Pocket export file (HTML).
  /// [fileBytes] are the raw bytes of the HTML file.
  /// [filename] is the name of the file (e.g., "ril_export.html").
  /// Returns an [ApiMessageWithLocation] containing the API message and the Location header of the import task.
  Future<ApiMessageWithLocation> importPocketFile(List<int> fileBytes, String filename) async {
    return _importMultipartFile('pocket-file', fileBytes, filename, fileContentType: MediaType('text', 'html'));
  }

  /// Imports bookmarks from a text file content.
  /// Corresponds to `POST /bookmarks/import/text`.
  /// Returns an [ApiMessageWithLocation] containing the API message and the Location header.
  Future<ApiMessageWithLocation> importTextFile(String textContent) async {
    final response = await _makeRawRequest(
      () => _httpClient.post(
        Uri.parse('$baseUrl/bookmarks/import/text'),
        headers: _getHeaders(contentType: 'text/plain; charset=utf-8', accept: 'application/json'),
        body: textContent,
      ),
    );
    final dynamic jsonBody = _decodeResponse(response, expectJson: true);
    final location = response.headers['location'];
     if (location == null) {
      throw ApiException("Import task accepted (202) but Location header missing.", statusCode: response.statusCode, responseBody: jsonBody);
    }
    return ApiMessageWithLocation(
      message: ApiMessage.fromJson(jsonBody as Map<String, dynamic>),
      location: location,
    );
  }

  /// Imports bookmarks from a Wallabag instance.
  /// Corresponds to `POST /bookmarks/import/wallabag`.
  /// Returns an [ApiMessageWithLocation] containing the API message and the Location header.
  Future<ApiMessageWithLocation> importWallabag(WallabagImport wallabagImportData) async {
     final response = await _makeRawRequest(
      () => _httpClient.post(
        Uri.parse('$baseUrl/bookmarks/import/wallabag'),
        headers: _getHeaders(),
        body: jsonEncode(wallabagImportData.toJson()),
      ),
    );
    final dynamic jsonBody = _decodeResponse(response, expectJson: true);
    final location = response.headers['location'];
     if (location == null) {
      throw ApiException("Import task accepted (202) but Location header missing.", statusCode: response.statusCode, responseBody: jsonBody);
    }
    return ApiMessageWithLocation(
      message: ApiMessage.fromJson(jsonBody as Map<String, dynamic>),
      location: location,
    );
  }

  /// Closes the underlying HTTP client.
  /// This should be called when the API client is no longer needed.
  void dispose() {
    _httpClient.close();
  }
}

/// Helper class to encapsulate an [ApiMessage] and an optional [location] URL,
/// typically from a 'Location' header in 201 or 202 responses.
class ApiMessageWithLocation {
  /// The API message.
  final ApiMessage message;
  /// The URL from the 'Location' header, if present.
  final String? location;

  ApiMessageWithLocation({required this.message, this.location});
}
