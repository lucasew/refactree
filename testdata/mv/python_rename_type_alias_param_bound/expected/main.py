class Assist:
    pass


type Vec[T: Assist] = list[T]


def use(v: Vec[Assist]) -> Assist:
    return v[0]
