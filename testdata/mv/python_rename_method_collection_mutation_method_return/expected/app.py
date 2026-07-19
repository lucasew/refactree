from collections import deque


class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


class BoxA:
    def __init__(self):
        self.a = A()

    def get(self) -> A:
        return self.a


class BoxB:
    def __init__(self):
        self.b = B()

    def get(self) -> B:
        return self.b


def use_append_mr(ba: BoxA, bb: BoxB):
    xs = []
    ys = []
    xs.append(ba.get())
    ys.append(bb.get())
    return xs[0].execute() + ys[0].run()


def use_append_mr_assign(ba: BoxA, bb: BoxB):
    xs = []
    ys = []
    xs.append(ba.get())
    ys.append(bb.get())
    xa = xs[0]
    xb = ys[0]
    return xa.execute() + xb.run()


def use_append_mr_for(ba: BoxA, bb: BoxB):
    xs = []
    ys = []
    xs.append(ba.get())
    ys.append(bb.get())
    n = 0
    for a in xs:
        n += a.execute()
    for b in ys:
        n += b.run()
    return n


def use_insert_mr(ba: BoxA, bb: BoxB):
    xs = []
    ys = []
    xs.insert(0, ba.get())
    ys.insert(0, bb.get())
    return xs[0].execute() + ys[0].run()


def use_extend_mr(ba: BoxA, bb: BoxB):
    xs = []
    ys = []
    xs.extend([ba.get()])
    ys.extend([bb.get()])
    return xs[0].execute() + ys[0].run()


def use_set_add_mr(ba: BoxA, bb: BoxB):
    xs = set()
    ys = set()
    xs.add(ba.get())
    ys.add(bb.get())
    return next(iter(xs)).execute() + next(iter(ys)).run()


def use_set_add_mr_for(ba: BoxA, bb: BoxB):
    xs = set()
    ys = set()
    xs.add(ba.get())
    ys.add(bb.get())
    n = 0
    for a in xs:
        n += a.execute()
    for b in ys:
        n += b.run()
    return n


def use_deque_append_mr(ba: BoxA, bb: BoxB):
    xs = deque()
    ys = deque()
    xs.append(ba.get())
    ys.append(bb.get())
    return xs[0].execute() + ys[0].run()


def use_deque_appendleft_mr(ba: BoxA, bb: BoxB):
    xs = deque()
    ys = deque()
    xs.appendleft(ba.get())
    ys.appendleft(bb.get())
    return xs[0].execute() + ys[0].run()


# Class regression — already worked.
def use_append_class():
    xs = []
    ys = []
    xs.append(A())
    ys.append(B())
    return xs[0].execute() + ys[0].run()


def use_set_add_class():
    xs = set()
    ys = set()
    xs.add(A())
    ys.add(B())
    return next(iter(xs)).execute() + next(iter(ys)).run()


def use_preserves_b(bb: BoxB):
    ys = []
    ys.append(bb.get())
    zs = set()
    zs.add(bb.get())
    dq = deque()
    dq.append(bb.get())
    return ys[0].run() + next(iter(zs)).run() + dq[0].run()
