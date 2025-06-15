import 'dart:convert';
import 'dart:io'; // For HttpHeaders
import 'package:http/http.dart' as http;
import 'package:mocktail/mocktail.dart'; // Import mocktail
import 'package:test/test.dart';

// Library and models
import 'package:readeck_client/readeck_api_client.dart';
import 'package:readeck_client/models.dart';

// Define a mock class for http.Client using mocktail
class MockHttpClient extends Mock implements http.Client {}

void main() {
  late MockHttpClient mockHttpClient;
  late ReadeckApiClient apiClient;
  const String baseUrl = 'http://fakeapi.com';

  setUpAll(() {
    // Fallback for when'registerFallbackValue' is needed for complex types
    // For basic types like Uri, String, Map, it's often not needed.
    // If we encounter issues with 'any' or 'captureAny', we might need this.
    // For example, if Uri was a custom class:
    // registerFallbackValue(Uri.parse('http://fallback.com'));
  });

  setUp(() {
    mockHttpClient = MockHttpClient();
    apiClient = ReadeckApiClient(baseUrl: baseUrl, httpClient: mockHttpClient);
  });

  group('ReadeckApiClient - Auth Endpoints', () {
    group('login', () {
      final authRequest = AuthRequest(
        username: 'testuser',
        password: 'password123',
        application: 'TestApp',
      );
      final requestBody = jsonEncode(authRequest.toJson());
      final expectedUri = Uri.parse('$baseUrl/auth');

      test('returns AuthResponse on successful login (200 OK) and sets token', () async {
        final mockAuthResponse = AuthResponse(id: 'token-id', token: 'sample-token');
        final responseBody = jsonEncode(mockAuthResponse.toJson());

        // Stubbing with mocktail: when(() => ...).thenAnswer(...)
        when(() => mockHttpClient.post(
          expectedUri,
          headers: any(named: 'headers'), // Use any(named: 'headers') for map
          body: requestBody,
        )).thenAnswer((_) async => http.Response(responseBody, 200));

        // For verifying internal token setting
        final profileUri = Uri.parse('$baseUrl/profile');
        final mockUserProfile = UserProfile(user: UserInfo(username: "test"));
        when(() => mockHttpClient.get(profileUri, headers: any(named: 'headers')))
            .thenAnswer((_) async => http.Response(jsonEncode(mockUserProfile.toJson()), 200));

        final result = await apiClient.login(authRequest);

        expect(result, isA<AuthResponse>());
        expect(result.token, 'sample-token');

        // Verify internal token setting by checking headers of a subsequent request
        await apiClient.getProfile();

        final captured = verify(() => mockHttpClient.get(profileUri, headers: captureAny(named: 'headers'))).captured;
        final capturedHeaders = captured.first as Map<String,String>; // mocktail captures a list

        expect(capturedHeaders[HttpHeaders.authorizationHeader], 'Bearer sample-token');
      });

      test('throws UnauthorizedException on 401 response', () async {
        final errorResponse = {'message': 'Invalid credentials'};
        final responseBody = jsonEncode(errorResponse);

        when(() => mockHttpClient.post(
          expectedUri,
          headers: any(named: 'headers'),
          body: requestBody,
        )).thenAnswer((_) async => http.Response(responseBody, 401));

        expect(
          () => apiClient.login(authRequest),
          throwsA(isA<UnauthorizedException>().having(
            (e) => e.message, 'message', 'Invalid credentials'
          )),
        );
      });

      test('throws ForbiddenException on 403 response', () async {
        final errorResponse = {'message': 'User account locked'};
        final responseBody = jsonEncode(errorResponse);

        when(() => mockHttpClient.post(
          expectedUri,
          headers: any(named: 'headers'),
          body: requestBody,
        )).thenAnswer((_) async => http.Response(responseBody, 403));

        expect(
            apiClient.login(authRequest),
            throwsA(isA<ForbiddenException>().having(
                (e) => e.message, 'message', 'User account locked'
            ))
        );
      });

      test('throws InternalServerErrorException on 500 response', () async {
        final errorResponse = {'message': 'Internal server error'};
        final responseBody = jsonEncode(errorResponse);

        when(() => mockHttpClient.post(
          expectedUri,
          headers: any(named: 'headers'),
          body: requestBody,
        )).thenAnswer((_) async => http.Response(responseBody, 500));

        expect(
            apiClient.login(authRequest),
            throwsA(isA<InternalServerErrorException>().having(
                (e) => e.message, 'message', 'Internal server error'
            ))
        );
      });
    });

    // Group for getProfile tests will be added next
  });
}
