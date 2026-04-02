// gRPC status codes returned by grpc-gateway in JSON error responses.
const grpcCodeText: Record<number, string> = {
  0: 'OK',
  1: 'Cancelled',
  2: 'Unknown',
  3: 'InvalidArgument',
  4: 'DeadlineExceeded',
  5: 'NotFound',
  6: 'AlreadyExists',
  7: 'PermissionDenied',
  8: 'ResourceExhausted',
  9: 'FailedPrecondition',
  10: 'Aborted',
  11: 'OutOfRange',
  12: 'Unimplemented',
  13: 'Internal',
  14: 'Unavailable',
  15: 'DataLoss',
  16: 'Unauthenticated',
};

export interface ApiError {
  code: number;
  message: string;
  details?: unknown[];
}

export class AdmiralError extends Error {
  readonly code: number;
  readonly status: string;
  readonly details: unknown[];

  constructor(apiError: ApiError) {
    super(apiError.message);
    this.name = 'AdmiralError';
    this.code = apiError.code;
    this.status = grpcCodeText[apiError.code] ?? 'Unknown';
    this.details = apiError.details ?? [];
  }
}

export function parseApiError(body: unknown): AdmiralError | undefined {
  if (body !== null && typeof body === 'object' && 'code' in body && 'message' in body) {
    return new AdmiralError(body as ApiError);
  }
  return undefined;
}
