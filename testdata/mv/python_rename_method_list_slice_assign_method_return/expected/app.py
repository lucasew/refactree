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


# List slice assign method-return under foreign same-leaf.
def use_slice_mr(ba: BoxA, bb: BoxB):
    xs_sm = []
    ys_sm = []
    xs_sm[:] = [ba.get()]
    ys_sm[:] = [bb.get()]
    return xs_sm[0].execute() + ys_sm[0].run()


def use_slice_mr_assign(ba: BoxA, bb: BoxB):
    xs_sma = []
    ys_sma = []
    xs_sma[:] = [ba.get()]
    ys_sma[:] = [bb.get()]
    xa = xs_sma[0]
    xb = ys_sma[0]
    return xa.execute() + xb.run()


def use_slice_mr_range(ba: BoxA, bb: BoxB):
    xs_smr = [None]
    ys_smr = [None]
    xs_smr[0:1] = [ba.get()]
    ys_smr[0:1] = [bb.get()]
    return xs_smr[0].execute() + ys_smr[0].run()


def use_slice_mr_tuple(ba: BoxA, bb: BoxB):
    xs_smt = []
    ys_smt = []
    xs_smt[:] = (ba.get(),)
    ys_smt[:] = (bb.get(),)
    return xs_smt[0].execute() + ys_smt[0].run()


def use_slice_mr_for(ba: BoxA, bb: BoxB):
    xs_smf = []
    ys_smf = []
    xs_smf[:] = [ba.get()]
    ys_smf[:] = [bb.get()]
    n = 0
    for a in xs_smf:
        n += a.execute()
    for b in ys_smf:
        n += b.run()
    return n


# Class regression — already worked.
def use_slice_class():
    xs_sc = []
    ys_sc = []
    xs_sc[:] = [A()]
    ys_sc[:] = [B()]
    return xs_sc[0].execute() + ys_sc[0].run()


def use_preserves_b(bb: BoxB):
    ys_pb = []
    ys_pb[:] = [bb.get()]
    return ys_pb[0].run()
