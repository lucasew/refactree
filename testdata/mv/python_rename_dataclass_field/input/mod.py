from dataclasses import dataclass


@dataclass
class Box:
    value: int
    count: int = 0


def use(b: Box) -> int:
    return b.value + b.count
