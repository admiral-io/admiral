export const ALLOWED_SSO_ERRORS = {
  invalid_token: 'Your session token could not be validated.',
  expired_token: 'Your session has expired. Please log in again.',
  invalid_state: 'Invalid authentication state. This may be due to an expired or tampered session.',
  missing_token: 'Authentication token is missing. Please try logging in again.',

  invalid_grant:
    'The authorization grant was rejected (for example invalid, expired, or not matching this app).',
  unauthorized_client: 'The client is not authorized to perform this action.',
  access_denied: 'Access was denied during the authentication process.',
  invalid_request: 'The authentication request was invalid or malformed.',
  invalid_scope: 'The requested scope is invalid or malformed.',
  server_error: 'An authentication server error occurred. Please try again later.',
  temporarily_unavailable:
    'The authentication service is temporarily unavailable. Please try again later.',

  session_expired: 'Your session has expired. Please log in again.',
  session_invalid: 'Your session is invalid. Please log in again.',
  logout_required: 'You have been logged out. Please log in again to continue.',

  network_error:
    'A network error occurred during authentication. Please check your connection and try again.',
  timeout: 'The authentication request timed out. Please try again.',

  misconfigured_client:
    'The authentication system is misconfigured. Please contact your administrator.',
  invalid_redirect_uri: 'Invalid redirect URL configuration. Please contact your administrator.',
} as const;

export type AllowedErrorType = keyof typeof ALLOWED_SSO_ERRORS;

export const AUTO_RETRY_ERRORS: ReadonlySet<AllowedErrorType> = new Set([
  'expired_token',
  'invalid_state',
  'missing_token',
  'session_expired',
  'session_invalid',
]);

export function extractErrorType(searchParams: URLSearchParams): {
  errorType: AllowedErrorType | null;
  rawMessage: string | null;
  isUnknown: boolean;
} {
  const message = searchParams.get('message');
  const error = searchParams.get('error');
  const errorDescription = searchParams.get('error_description');

  const rawMessage = message || error || errorDescription;

  if (!rawMessage) {
    return { errorType: null, rawMessage: null, isUnknown: false };
  }

  const normalizedMessage = rawMessage.toLowerCase().trim();

  const exactMatch = Object.keys(ALLOWED_SSO_ERRORS).find((key) => normalizedMessage === key) as
    | AllowedErrorType
    | undefined;

  if (exactMatch) {
    return { errorType: exactMatch, rawMessage, isUnknown: false };
  }

  const partialMatch = Object.keys(ALLOWED_SSO_ERRORS).find((key) => {
    const keyWithSpaces = key.replaceAll('_', ' ');
    return (
      normalizedMessage.includes(keyWithSpaces) ||
      normalizedMessage.includes(key) ||
      (key === 'expired_token' &&
        (normalizedMessage.includes('expired') || normalizedMessage.includes('token'))) ||
      (key === 'invalid_token' &&
        normalizedMessage.includes('invalid') &&
        normalizedMessage.includes('token')) ||
      (key === 'invalid_state' && normalizedMessage.includes('state')) ||
      (key === 'access_denied' && normalizedMessage.includes('denied')) ||
      (key === 'session_expired' && normalizedMessage.includes('session'))
    );
  }) as AllowedErrorType | undefined;

  if (partialMatch) {
    return { errorType: partialMatch, rawMessage, isUnknown: false };
  }

  return { errorType: null, rawMessage, isUnknown: true };
}
