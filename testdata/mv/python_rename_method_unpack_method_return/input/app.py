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


# List/tuple unpack method-return under foreign same-leaf.
def use_list_unpack_mr(ba: BoxA, bb: BoxB):
    [mrA] = [ba.get()]
    [mrB] = [bb.get()]
    return mrA.run() + mrB.run()


def use_tuple_unpack_mr(ba: BoxA, bb: BoxB):
    (mrA,) = (ba.get(),)
    (mrB,) = (bb.get(),)
    return mrA.run() + mrB.run()


def use_seq_unpack_mr(ba: BoxA, bb: BoxB):
    mrA, = [ba.get()]
    mrB, = [bb.get()]
    return mrA.run() + mrB.run()


def use_multi_unpack_mr(ba: BoxA, bb: BoxB):
    mrA0, mrA1 = ba.get(), ba.get()
    mrB0, mrB1 = bb.get(), bb.get()
    return mrA0.run() + mrA1.run() + mrB0.run() + mrB1.run()


# Class regression — already worked.
def use_list_unpack_class():
    [classA] = [A()]
    [classB] = [B()]
    return classA.run() + classB.run()


def use_preserves_b(bb: BoxB):
    [mrB] = [bb.get()]
    return mrB.run()
