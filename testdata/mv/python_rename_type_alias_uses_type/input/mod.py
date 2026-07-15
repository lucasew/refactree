class Box:
    pass

type BoxList = list[Box]

def use(x: BoxList) -> Box:
    return Box()
