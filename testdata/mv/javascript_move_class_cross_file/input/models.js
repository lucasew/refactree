export class Config {
  constructor(name) {
    this.name = name;
  }
}

export function createConfig(name) {
  return new Config(name);
}
