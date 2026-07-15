export class Assist {
  n = 1;
}
export class Stay {
  n = 2;
}
export function use(c) {
  return new (c ? Assist : Stay)();
}
