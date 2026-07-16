from box import Box


def make() -> Box:
    return Box(helper=1, stay=2)


def use(b: Box) -> int:
    return b.helper + b.stay
