from dataclasses import dataclass


@dataclass
class Box:
    assist: int
    stay: int = 0


def make() -> Box:
    return Box(assist=1, stay=2)


def use(b: Box) -> int:
    return b.assist + b.stay
