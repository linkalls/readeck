import 'dart:convert';
import 'dart:io';
import 'package:http/http.dart' as http;
import 'package:mockito/annotations.dart';
import 'package:mockito/mockito.dart';
import 'package:test/test.dart';

// Client and models
import 'package:readeck_client/readeck_api_client.dart';
import 'package:readeck_client/models.dart';

// Part directive for mocks will be at the end

void main() {
  late MockClient mockHttpClient;
  late ReadeckApiClient apiClient;
  const String baseUrl = 'http://fakeapi.com';

  setUp(() {
    mockHttpClient = MockClient();
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
        final mockAuthResponse = AuthResponse(id: 'user-id-123', token: 'sample-token');
        final responseBody = jsonEncode(mockAuthResponse.toJson());

        when(mockHttpClient.post(
          expectedUri,
          headers: anyNamed('headers'),
          body: requestBody,
        )).thenAnswer((_) async => http.Response(responseBody, 200));

        final result = await apiClient.login(authRequest);

        expect(result, isA<AuthResponse>());
        expect(result.token, 'sample-token');

        final userProfileJson = UserProfile(user: UserInfo(username: "testuser")).toJson();
        when(mockHttpClient.get(Uri.parse('$baseUrl/profile'), headers: anyNamed('headers')))
            .thenAnswer((_) async => http.Response(jsonEncode(userProfileJson), 200));

        await apiClient.getProfile();

        final captured = verify(mockHttpClient.get(
            Uri.parse('$baseUrl/profile'),
            headers: captureAnyNamed('headers')
        )).captured;

        expect(captured, isNotEmpty);
        expect(captured.single, isA<Map<String,String>>());
        final capturedHeaders = captured.single as Map<String,String>;
        expect(capturedHeaders[HttpHeaders.authorizationHeader], 'Bearer sample-token');
      });

      test('throws UnauthorizedException on 401 response', () async {
        final errorResponse = {'message': 'Invalid credentials'};
        final responseBody = jsonEncode(errorResponse);

        when(mockHttpClient.post(
          expectedUri,
          headers: anyNamed('headers'),
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

        when(mockHttpClient.post(
          expectedUri,
          headers: anyNamed('headers'),
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

        when(mockHttpClient.post(
          expectedUri,
          headers: anyNamed('headers'),
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
  });
}

// Generate mocks for http.Client
@GenerateMocks([http.Client])
part 'readeck_api_client_test.mocks.dart'; // Moved to the end of the file
