import copy


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


class BoxA:
    item: A

    def __init__(self, item: A):
        self.item = item

    def get(self) -> A:
        return self.item


class BoxB:
    item: B

    def __init__(self, item: B):
        self.item = item

    def get(self) -> B:
        return self.item


def use_obj(ba: BoxA, bb: BoxB) -> int:
    return copy.copy(ba.get()).run() + copy.deepcopy(bb.get()).run()


def use_list(ba: BoxA, bb: BoxB) -> int:
    return copy.copy([ba.get()])[0].run() + copy.copy([bb.get()])[0].run()


def use_deepcopy_list(ba: BoxA, bb: BoxB) -> int:
    return copy.deepcopy([ba.get()])[0].run() + copy.deepcopy([bb.get()])[0].run()


def use_assign(ba: BoxA, bb: BoxB) -> int:
    xa = copy.copy(ba.get())
    xb = copy.deepcopy(bb.get())
    return xa.run() + xb.run()


def use_list_assign(ba: BoxA, bb: BoxB) -> int:
    xs = copy.copy([ba.get()])
    ys = copy.copy([bb.get()])
    return xs[0].run() + ys[0].run()


def use_class() -> int:
    return copy.copy(A()).run() + copy.copy(B()).run()


def use_preserves_b(bb: BoxB) -> int:
    return copy.copy(bb.get()).run() + copy.copy([bb.get()])[0].run()
