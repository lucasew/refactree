from dataclasses import dataclass


@dataclass
class Crate:
    value: int
    count: int = 0


def use(b: Crate) -> int:
    return b.value + b.count
