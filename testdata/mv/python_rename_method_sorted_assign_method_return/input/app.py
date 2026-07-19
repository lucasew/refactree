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


def use_direct(ba: BoxA, bb: BoxB) -> int:
    return sorted([ba.get()])[0].run() + sorted([bb.get()])[0].run()


def use_assign_elem(ba: BoxA, bb: BoxB) -> int:
    xa = sorted([ba.get()])[0]
    xb = sorted([bb.get()])[0]
    return xa.run() + xb.run()


def use_assign_list(ba: BoxA, bb: BoxB) -> int:
    xs = sorted([ba.get()])
    ys = sorted([bb.get()])
    return xs[0].run() + ys[0].run()


def use_list_idx(ba: BoxA, bb: BoxB) -> int:
    xa = [ba.get()][0]
    xb = [bb.get()][0]
    return xa.run() + xb.run()


def use_preserves_b(bb: BoxB) -> int:
    xa = sorted([bb.get()])[0]
    return xa.run()
