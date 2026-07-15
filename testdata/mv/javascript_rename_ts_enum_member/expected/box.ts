export enum Color {
  Assist = 1,
  Stay = 2,
}
export function use(): number {
  return Color.Assist + Color.Stay;
}
