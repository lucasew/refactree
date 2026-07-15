type BoxAlias = list[int]

class Box:
    pass

def use(x: BoxAlias) -> Box:
    return Box()
