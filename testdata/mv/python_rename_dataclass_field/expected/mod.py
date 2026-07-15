from dataclasses import dataclass


@dataclass
class Box:
    amount: int
    count: int = 0


def use(b: Box) -> int:
    return b.amount + b.count
