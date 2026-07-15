export class Box {
  assist() { return this }
  stay() { return 2 }
  next() { return this }
}
export function use(b) {
  return b.next().assist().stay()
}
