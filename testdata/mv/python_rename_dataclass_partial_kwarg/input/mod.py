from dataclasses import dataclass
from functools import partial


@dataclass
class Box:
    helper: int
    stay: int = 0


make = partial(Box, helper=1)


def use() -> Box:
    return make(stay=2)
