from dataclasses import dataclass
from functools import partial


@dataclass
class Box:
    assist: int
    stay: int = 0


make = partial(Box, assist=1)


def use() -> Box:
    return make(stay=2)
