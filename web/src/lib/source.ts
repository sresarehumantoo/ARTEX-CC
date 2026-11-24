export const SOURCE_URL = 'https://github.com/SREsAreHumanToo/ARTEX-CC'

// AGPL-3.0 section 13: users of the network service must be offered the
// Corresponding Source, so link the exact tag when the build carries one.
export function sourceUrlFor(version?: string): string {
  return version && /^v\d+\.\d+\.\d+/.test(version) ? `${SOURCE_URL}/tree/${version}` : SOURCE_URL
}
