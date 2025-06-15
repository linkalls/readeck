import 'dart:convert';
import 'dart:io'; // For HttpHeaders
import 'dart:typed_data'; // For Uint8List

import 'package:http/http.dart' as http;

// Models
import 'models/auth.dart';
import 'models/profile.dart';
import 'models/common.dart';
import 'models/bookmark.dart';
import 'models/label.dart'; // Added Label models
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

  Map<String, String> _getHeaders({String accept = 'application/json'}) {
    final headers = <String, String>{
      HttpHeaders.contentTypeHeader: 'application/json; charset=utf-8',
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
        throw ApiException(
            'Expected JSON response but failed to decode. Body: ${utf8.decode(response.bodyBytes)}',
            statusCode: response.statusCode,
            responseBody: utf8.decode(response.bodyBytes)
        );
      }
    }
    final contentType = response.headers[HttpHeaders.contentTypeHeader];
    if (contentType != null && (contentType.contains('text/html') || contentType.contains('text/markdown'))) {
        return utf8.decode(response.bodyBytes);
    }
    if (contentType != null && contentType.contains('application/epub+zip')) {
        return response.bodyBytes;
    }
    return utf8.decode(response.bodyBytes);
  }

  Future<dynamic> _handleResponse(http.Response response, {bool expectJson = true}) async {
    if (response.statusCode >= 200 && response.statusCode < 300 && !expectJson) {
        return _decodeResponse(response, expectJson: false);
    }
    final dynamic decodedBody = _decodeResponse(response, expectJson: true); // Errors are usually JSON

    if (response.statusCode >= 200 && response.statusCode < 300) {
      return decodedBody; // This path is now only for expectJson = true
    } else if (response.statusCode == 401) {
      throw UnauthorizedException(
        decodedBody is Map ? decodedBody['message'] ?? 'Unauthorized' : decodedBody?.toString() ?? 'Unauthorized',
        responseBody: decodedBody
      );
    } else if (response.statusCode == 403) {
      throw ForbiddenException(
        decodedBody is Map ? decodedBody['message'] ?? 'Forbidden' : decodedBody?.toString() ?? 'Forbidden',
        responseBody: decodedBody
      );
    } else if (response.statusCode == 404) {
      throw NotFoundException(
        decodedBody is Map ? decodedBody['message'] ?? 'Not Found' : decodedBody?.toString() ?? 'Not Found',
        responseBody: decodedBody
      );
    } else if (response.statusCode == 422) {
      final apiError = decodedBody is Map && decodedBody.containsKey('isValid') ? ApiError.fromJson(decodedBody.cast<String,dynamic>()) : null;
      throw ValidationException(
        apiError?.message ?? (decodedBody is Map ? decodedBody['message'] ?? 'Validation Error' : decodedBody?.toString() ?? 'Validation Error'),
        errors: apiError?.fields?.map((key, value) => MapEntry(key, value.errors ?? [])),
        responseBody: decodedBody
      );
    } else if (response.statusCode >= 500 && response.statusCode < 600) {
      throw InternalServerErrorException(
        decodedBody is Map ? decodedBody['message'] ?? 'Internal Server Error' : decodedBody?.toString() ?? 'Internal Server Error',
        responseBody: decodedBody
      );
    }
     else {
      throw ApiException(
        decodedBody is Map ? decodedBody['message'] ?? 'API Error' : decodedBody?.toString() ?? 'API Error: ${response.statusCode}',
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
    if (body == null && null is! T && expectJsonResponse) { // check if T is non-nullable
        throw ApiException('Expected JSON response for ${T.toString()}, but received null body', statusCode: response.statusCode);
    }
    return fromBody(body);
  }

  Future<void> _makeRequestVoidReturn( // Renamed from _makeRequestNoResponse for clarity
    Future<http.Response> Function() requestFunction,
  ) async {
    final response = await requestFunction();
    await _handleResponse(response); // Errors will be thrown, 204 will result in null and complete.
  }

  Map<String, String> _buildQueryParameters(Map<String, dynamic> params) {
    return params.entries
        .where((entry) => entry.value != null)
        .map((entry) {
          if (entry.value is List) {
            if ((entry.value as List).isEmpty) return MapEntry(entry.key, '');
            return MapEntry(entry.key, (entry.value as List).join(','));
          }
          return MapEntry(entry.key, entry.value.toString());
        })
        .fold<Map<String, String>>({}, (map, entry) {
           if (entry.value.isNotEmpty || entry.key == 'labels') { // Allow 'labels=' for empty label list
            map[entry.key] = entry.value;
          }
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
    final response = await _httpClient.post(
      Uri.parse('$baseUrl/bookmarks'),
      headers: _getHeaders(),
      body: jsonEncode(bookmarkCreate.toJson()),
    );
    final dynamic responseBody = await _handleResponse(response, expectJson: true);

    if (response.statusCode == 202) {
      final bookmarkId = response.headers['bookmark-id'];
      if (bookmarkId != null) {
        return getBookmark(bookmarkId);
      } else {
        if (responseBody is Map && responseBody['message'] != null) {
             throw ApiException(
                "Bookmark creation initiated (202), 'bookmark-id' header missing. Message: ${responseBody['message']}",
                statusCode: response.statusCode, responseBody: responseBody);
        }
        throw ApiException(
            "Bookmark creation initiated (202), but no 'bookmark-id' header found and body is not a standard message.",
            statusCode: response.statusCode, responseBody: responseBody);
      }
    }
    if (responseBody is Map<String, dynamic>) {
        return BookmarkInfo.fromJson(responseBody);
    }
    throw ApiException(
        "Unexpected response after creating bookmark. Status: ${response.statusCode}",
        statusCode: response.statusCode, responseBody: responseBody);
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
      (json) => (json as List) // Assuming API returns a list even for a single named label info
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

  void dispose() {
    _httpClient.close();
  }
}
