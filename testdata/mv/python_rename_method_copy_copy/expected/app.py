import copy


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_obj(item: A, other: B) -> int:
    a = copy.copy(item)
    b = copy.copy(other)
    return a.execute() + b.run()


def use_deepcopy(item: A, other: B) -> int:
    a = copy.deepcopy(item)
    b = copy.deepcopy(other)
    return a.execute() + b.run()


def use_coll(items: list[A], others: list[B]) -> int:
    xs = copy.copy(items)
    ys = copy.copy(others)
    total = 0
    for a in xs:
        total += a.execute()
    for b in ys:
        total += b.run()
    a2 = copy.copy(items)[0]
    b2 = copy.copy(others)[0]
    return total + a2.execute() + b2.run()


def use_coll_for(items: list[A], others: list[B]) -> int:
    total = 0
    for a in copy.copy(items):
        total += a.execute()
    for b in copy.copy(others):
        total += b.run()
    return total
