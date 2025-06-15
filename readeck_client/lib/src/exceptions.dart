// Base class for all API related exceptions
class ApiException implements Exception {
  final String message;
  final int? statusCode;
  final dynamic responseBody; // Can hold decoded JSON or raw string

  ApiException(this.message, {this.statusCode, this.responseBody});

  @override
  String toString() {
    return 'ApiException: $message (Status Code: $statusCode)';
  }
}

class UnauthorizedException extends ApiException {
  UnauthorizedException(String message, {dynamic responseBody})
      : super(message, statusCode: 401, responseBody: responseBody);
}

class ForbiddenException extends ApiException {
  ForbiddenException(String message, {dynamic responseBody})
      : super(message, statusCode: 403, responseBody: responseBody);
}

class NotFoundException extends ApiException {
  NotFoundException(String message, {dynamic responseBody})
      : super(message, statusCode: 404, responseBody: responseBody);
}

class ValidationException extends ApiException {
  // TODO: Consider adding a field for structured error details (e.g., Map<String, List<String>>)
  final Map<String, dynamic>? errors; // To hold structured validation errors

  ValidationException(String message, {this.errors, dynamic responseBody})
      : super(message, statusCode: 422, responseBody: responseBody);

  @override
  String toString() {
    return 'ValidationException: $message (Status Code: $statusCode) Errors: $errors';
  }
}

class InternalServerErrorException extends ApiException {
  InternalServerErrorException(String message, {dynamic responseBody})
      : super(message, statusCode: 500, responseBody: responseBody);
}
