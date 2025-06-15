import 'dart:convert';
import 'dart:io'; // For HttpHeaders
import 'dart:typed_data'; // For Uint8List

import 'package:http/http.dart' as http;

// Models
import 'models/auth.dart';
import 'models/profile.dart';
import 'models/common.dart';
import 'models/bookmark.dart';
import 'models/label.dart';
import 'models/annotation.dart';
import 'models/collection.dart';
import 'models/import_models.dart'; // Added Import models
import 'exceptions.dart';

class ReadeckApiClient {
  final String baseUrl;
  String? _token;
  http.Client _httpClient;

  ReadeckApiClient({required this.baseUrl, String? token, http.Client? httpClient})
      : _token = token,
        _httpClient = httpClient ?? http.Client();

  void setToken(String token) {
    _token = token;
  }

  void clearToken() {
    _token = null;
  }

  Map<String, String> _getHeaders({String contentType = 'application/json; charset=utf-8', String accept = 'application/json'}) {
    final headers = <String, String>{
      HttpHeaders.contentTypeHeader: contentType,
      HttpHeaders.acceptHeader: accept,
    };
    if (_token != null) {
      headers[HttpHeaders.authorizationHeader] = 'Bearer $_token';
    }
    return headers;
  }

  dynamic _decodeResponse(http.Response response, {bool expectJson = true}) {
    if (response.bodyBytes.isEmpty) {
      return null;
    }
    if (expectJson) {
      try {
        return jsonDecode(utf8.decode(response.bodyBytes));
      } catch (e) {
        // Return raw body for further diagnostics in _handleResponse if JSON parse fails
        return utf8.decode(response.bodyBytes);
      }
    }
    final contentTypeHeader = response.headers[HttpHeaders.contentTypeHeader];
    if (contentTypeHeader != null && (contentTypeHeader.contains('text/html') || contentTypeHeader.contains('text/markdown'))) {
        return utf8.decode(response.bodyBytes);
    }
    if (contentTypeHeader != null && contentTypeHeader.contains('application/epub+zip')) {
        return response.bodyBytes;
    }
    return utf8.decode(response.bodyBytes);
  }

  Future<dynamic> _handleResponse(http.Response response, {bool expectJson = true}) async {
    dynamic decodedBody;

    if (response.statusCode >= 200 && response.statusCode < 300 && !expectJson) {
        return _decodeResponse(response, expectJson: false);
    }

    // For JSON success responses or all error responses (which are typically JSON)
    // Attempt to decode as JSON first.
    decodedBody = _decodeResponse(response, expectJson: true);
    if (decodedBody is String && expectJson && !(response.statusCode >= 200 && response.statusCode < 300) ) {
      // If JSON was expected, but decoding returned a string (meaning parsing failed),
      // and it's an error status, wrap it in a generic message for the exception.
      // For success status with expectJson=true, if it's a string, it's an API contract violation.
       throw ApiException(
            'Expected JSON response but received non-JSON. Body: $decodedBody',
            statusCode: response.statusCode,
            responseBody: decodedBody
        );
    }


    if (response.statusCode >= 200 && response.statusCode < 300) {
      return decodedBody;
    } else if (response.statusCode == 401) {
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
      final apiError = decodedBody is Map ? ApiError.fromJson(decodedBody.cast<String,dynamic>()) : null;
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

  Future<T> _makeRequest<T>(
    Future<http.Response> Function() requestFunction,
    T Function(dynamic body) fromBody, {bool expectJsonResponse = true}
  ) async {
    final response = await requestFunction();
    final dynamic body = await _handleResponse(response, expectJson: expectJsonResponse);
    if (body == null && null is! T && expectJsonResponse) {
        throw ApiException('Expected response body for ${T.toString()}, but received null', statusCode: response.statusCode);
    }
    return fromBody(body);
  }

  Future<http.Response> _makeRawRequest(
    Future<http.Response> Function() requestFunction,
  ) async {
    final response = await requestFunction();
    // Perform basic error checking without trying to fully parse/decode body yet
    // This ensures that we don't try to access headers on a completely failed request too early.
    if (response.statusCode >= 400) {
        await _handleResponse(response, expectJson: true); // Will throw appropriate exception
    }
    return response; // Return raw response for caller to process body and headers
  }

  Future<void> _makeRequestVoidReturn( // Changed from _makeRequestNoResponse
    Future<http.Response> Function() requestFunction,
  ) async {
    final response = await requestFunction();
    await _handleResponse(response, expectJson: false);
  }

  Map<String, String> _buildQueryParameters(Map<String, dynamic> params) {
    return params.entries
        .where((entry) {
          if (entry.key == 'labels') return true;
          return entry.value != null;
        })
        .map((entry) {
          if (entry.value is List) {
            return MapEntry(entry.key, (entry.value as List).join(','));
          }
          return MapEntry(entry.key, entry.value?.toString() ?? '');
        })
        .fold<Map<String, String>>({}, (map, entry) {
          map[entry.key] = entry.value;
          return map;
        });
  }

  // --- Auth Endpoints ---
  Future<AuthResponse> login(AuthRequest authRequest) async {
    return _makeRequest(
      () => _httpClient.post(
        Uri.parse('$baseUrl/auth'),
        headers: _getHeaders(),
        body: jsonEncode(authRequest.toJson()),
      ),
      (json) => AuthResponse.fromJson(json as Map<String, dynamic>),
    );
  }

  Future<UserProfile> getProfile() async {
    return _makeRequest(
      () => _httpClient.get(
        Uri.parse('$baseUrl/profile'),
        headers: _getHeaders(),
      ),
      (json) => UserProfile.fromJson(json as Map<String, dynamic>),
    );
  }

  Future<void> logout() async {
    return _makeRequestVoidReturn(
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
    // This method expects 202 Accepted, with Location and Bookmark-Id headers.
    // The body of 202 is typically an ApiMessage.
    final response = await _makeRawRequest(
      () => _httpClient.post(
        Uri.parse('$baseUrl/bookmarks'),
        headers: _getHeaders(),
        body: jsonEncode(bookmarkCreate.toJson()),
      ),
    );
    // _makeRawRequest already called _handleResponse for status >= 400
    // Now check status code for 202 specific logic
    if (response.statusCode == 202) {
        final bookmarkId = response.headers['bookmark-id'];
        // final location = response.headers['location']; // Available if needed
        final apiMessageBody = _decodeResponse(response, expectJson: true); // Decode the ApiMessage body

        if (bookmarkId != null) {
            // Successfully initiated, now fetch the actual bookmark
            return getBookmark(bookmarkId);
        } else {
            throw ApiException(
                "Bookmark creation initiated (202), but 'bookmark-id' header was missing.",
                statusCode: response.statusCode,
                responseBody: apiMessageBody);
        }
    } else {
        // If not 202, but still 2xx (e.g. 201 if API behavior changes)
        final dynamic body = _decodeResponse(response, expectJson: true);
        if (body is Map<String,dynamic>) {
             return BookmarkInfo.fromJson(body);
        }
        throw ApiException(
            "Unexpected success status code: ${response.statusCode} after creating bookmark.",
            statusCode: response.statusCode,
            responseBody: body);
    }
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
  Future<ApiMessageWithLocation> importTextFile(String textContent) async {
    final response = await _makeRawRequest(
      () => _httpClient.post(
        Uri.parse('$baseUrl/bookmarks/import/text'),
        headers: _getHeaders(contentType: 'text/plain; charset=utf-8', accept: 'application/json'),
        body: textContent,
      ),
    );
    // _makeRawRequest has already checked for >=400 status codes.
    final dynamic jsonBody = _decodeResponse(response, expectJson: true);
    final location = response.headers['location'];
    return ApiMessageWithLocation(
      message: ApiMessage.fromJson(jsonBody as Map<String, dynamic>),
      location: location,
    );
  }

  Future<ApiMessageWithLocation> importWallabag(WallabagImport wallabagImportData) async {
     final response = await _makeRawRequest(
      () => _httpClient.post(
        Uri.parse('$baseUrl/bookmarks/import/wallabag'),
        headers: _getHeaders(), // Default Content-Type: application/json, Accept: application/json
        body: jsonEncode(wallabagImportData.toJson()),
      ),
    );
    final dynamic jsonBody = _decodeResponse(response, expectJson: true);
    final location = response.headers['location'];
    return ApiMessageWithLocation(
      message: ApiMessage.fromJson(jsonBody as Map<String, dynamic>),
      location: location,
    );
  }

  void dispose() {
    _httpClient.close();
  }
}

// Helper class for responses that include a Location header along with ApiMessage
class ApiMessageWithLocation {
  final ApiMessage message;
  final String? location;

  ApiMessageWithLocation({required this.message, this.location});
}
