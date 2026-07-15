export class Box {
  helper() { return this }
  stay() { return 2 }
  next() { return this }
}
export function use(b) {
  return b.next().helper().stay()
}
