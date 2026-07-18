from copy import copy, deepcopy


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_obj(item: A, other: B) -> int:
    a = copy(item)
    b = copy(other)
    return a.run() + b.run()


def use_deepcopy(item: A, other: B) -> int:
    a = deepcopy(item)
    b = deepcopy(other)
    return a.run() + b.run()


def use_walrus(item: A, other: B) -> int:
    if (a := copy(item)):
        return a.run()
    if (b := deepcopy(other)):
        return b.run()
    return 0


def use_coll(items: list[A], others: list[B]) -> int:
    xs = copy(items)
    ys = copy(others)
    total = 0
    for a in xs:
        total += a.run()
    for b in ys:
        total += b.run()
    a2 = copy(items)[0]
    b2 = copy(others)[0]
    return total + a2.run() + b2.run()


def use_coll_for(items: list[A], others: list[B]) -> int:
    total = 0
    for a in copy(items):
        total += a.run()
    for b in deepcopy(others):
        total += b.run()
    return total
