from box import Box


def make() -> Box:
    return Box(assist=1, stay=2)


def use(b: Box) -> int:
    return b.assist + b.stay
