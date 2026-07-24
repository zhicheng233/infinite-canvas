export class ApiError extends Error {
  errorDetail?: string;
  constructor(message: string, errorDetail?: string) {
    super(message);
    this.name = "ApiError";
    this.errorDetail = errorDetail;
  }
}
