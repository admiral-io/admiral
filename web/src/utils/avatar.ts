/**
 * Validates if a URL string is valid and non-empty
 * @param url - The URL string to validate
 * @returns The URL if valid, undefined otherwise
 */
export const getValidPictureUrl = (url?: string): string | undefined => {
  if (!url || url.trim() === '') return undefined;
  try {
    const parsed = new URL(url);
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return undefined;
    return url;
  } catch {
    return undefined;
  }
};

/**
 * Gets the initial character for avatar fallback
 * @param name - Primary name
 * @param email - Fallback email if name is not available
 * @returns The uppercase first character or '?'
 */
export const getAvatarInitial = (name?: string, email?: string): string => {
  return (name || email || '?').charAt(0).toUpperCase();
};
