import { Crate, Sub } from './box.js';
export const b = new Crate(1);
export const s = new Sub(2);
export function take(x) {
  return x instanceof Crate;
}
