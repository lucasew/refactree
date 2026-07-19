class A:
    def run(self):
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


# Class regression — already solid.
def gen_class_a():
    yield A()


def gen_class_b():
    yield B()


# Method-return yields under foreign same-leaf.
def gen_mr_param_a(ba: BoxA):
    yield ba.get()


def gen_mr_param_b(bb: BoxB):
    yield bb.get()


def gen_mr_local_a():
    ba = BoxA()
    yield ba.get()


def gen_mr_local_b():
    bb = BoxB()
    yield bb.get()


def gen_mr_assign_a(ba: BoxA):
    xa = ba.get()
    yield xa


def gen_mr_assign_b(bb: BoxB):
    xb = bb.get()
    yield xb


def use_class_next():
    return next(gen_class_a()).run() + next(gen_class_b()).run()


def use_mr_param_next(ba: BoxA, bb: BoxB):
    return next(gen_mr_param_a(ba)).run() + next(gen_mr_param_b(bb)).run()


def use_mr_local_next():
    return next(gen_mr_local_a()).run() + next(gen_mr_local_b()).run()


def use_mr_assign_next(ba: BoxA, bb: BoxB):
    return next(gen_mr_assign_a(ba)).run() + next(gen_mr_assign_b(bb)).run()


def use_class_for():
    r = 0
    for class_xa in gen_class_a():
        r += class_xa.run()
    for class_xb in gen_class_b():
        r += class_xb.run()
    return r


def use_mr_param_for(ba: BoxA, bb: BoxB):
    r = 0
    for mr_xa in gen_mr_param_a(ba):
        r += mr_xa.run()
    for mr_xb in gen_mr_param_b(bb):
        r += mr_xb.run()
    return r


def use_class_list():
    return list(gen_class_a())[0].run() + list(gen_class_b())[0].run()


def use_mr_param_list(ba: BoxA, bb: BoxB):
    return list(gen_mr_param_a(ba))[0].run() + list(gen_mr_param_b(bb))[0].run()


def use_class_assign():
    class_xa = next(gen_class_a())
    class_xb = next(gen_class_b())
    return class_xa.run() + class_xb.run()


def use_mr_param_assign(ba: BoxA, bb: BoxB):
    mr_xa = next(gen_mr_param_a(ba))
    mr_xb = next(gen_mr_param_b(bb))
    return mr_xa.run() + mr_xb.run()


def use_preserves_b(bb: BoxB):
    return next(gen_mr_param_b(bb)).run() + next(gen_class_b()).run()
