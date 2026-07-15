export class Box {
  assist() {
    return 1;
  }

  stay() {
    return 2;
  }
}

export function use({ assist, stay }) {
  return assist() + stay();
}
