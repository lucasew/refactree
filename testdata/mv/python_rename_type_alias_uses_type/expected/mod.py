class Crate:
    pass

type BoxList = list[Crate]

def use(x: BoxList) -> Crate:
    return Crate()
