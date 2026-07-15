from dataclasses import dataclass, replace


@dataclass
class Box:
    helper: int
    stay: int = 0


def use(b: Box) -> Box:
    return replace(b, helper=3, stay=b.stay)
