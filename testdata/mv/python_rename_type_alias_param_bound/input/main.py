class Helper:
    pass


type Vec[T: Helper] = list[T]


def use(v: Vec[Helper]) -> Helper:
    return v[0]
