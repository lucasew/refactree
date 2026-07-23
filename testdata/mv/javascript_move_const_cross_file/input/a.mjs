const engines = '>=22.12.0';
const skip = 23;

export function check(version) {
  return version + engines + skip;
}
