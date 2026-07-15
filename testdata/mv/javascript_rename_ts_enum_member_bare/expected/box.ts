export enum Color {
  Assist,
  Stay,
}
export function use(): number {
  return Color.Assist + Color.Stay;
}
