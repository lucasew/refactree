export class Box {
  helper() {
    return 1;
  }

  stay() {
    return 2;
  }
}

export function use({ helper, stay }) {
  return helper() + stay();
}
