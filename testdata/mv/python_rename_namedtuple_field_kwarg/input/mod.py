from typing import NamedTuple


class Box(NamedTuple):
    helper: int
    stay: int


def make() -> Box:
    return Box(helper=1, stay=2)


def use(b: Box) -> int:
    return b.helper + b.stay
