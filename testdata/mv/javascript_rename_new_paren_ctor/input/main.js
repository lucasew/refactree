export class Helper {
  n = 1;
}
export class Stay {
  n = 2;
}
export function use(c) {
  return new (c ? Helper : Stay)();
}
