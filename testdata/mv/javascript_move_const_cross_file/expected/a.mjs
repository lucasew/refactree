import { engines } from './b';
const skip = 23;

export function check(version) {
  return version + engines + skip;
}
