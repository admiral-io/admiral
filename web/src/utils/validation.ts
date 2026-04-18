// Rules: 1–63 characters, starts with a lowercase letter, ends with a
// lowercase letter or digit, only lowercase letters, digits, and hyphens
// in between. This mirrors DNS label / CLI-friendly naming conventions.
const RESOURCE_NAME_PATTERN = /^[a-z]([a-z0-9-]{0,61}[a-z0-9])?$/;

const RESOURCE_NAME_HINT = 'a\u2013z, 0\u20139, and hyphens (e.g. my-app-1)';

export function validateResourceName(value: string): string | undefined {
  if (!value) return undefined;
  if (RESOURCE_NAME_PATTERN.test(value)) return undefined;

  if (/[A-Z]/.test(value)) return 'Name must be lowercase.';
  if (/^[^a-z]/.test(value)) return 'Must start with a lowercase letter.';
  if (/[^a-z0-9-]/.test(value)) return 'Only lowercase letters, numbers, and hyphens are allowed.';
  if (/-$/.test(value)) return 'Must end with a letter or number, not a hyphen.';
  if (value.length > 63) return 'Name must be 63 characters or fewer.';

  return 'Invalid name format. Use lowercase letters, numbers, and hyphens (e.g. my-app-1).';
}

export { RESOURCE_NAME_PATTERN, RESOURCE_NAME_HINT };
