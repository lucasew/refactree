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


# Ternary-assign method-return under foreign same-leaf.
def use_ternary_assign_mr(c, ba: BoxA, bb: BoxB):
    mrA = ba.get() if c else ba.get()
    mrB = bb.get() if c else bb.get()
    return mrA.execute() + mrB.run()


def use_ternary_assign_mr_paren(c, ba: BoxA, bb: BoxB):
    mrA = (ba.get() if c else ba.get())
    mrB = (bb.get() if c else bb.get())
    return mrA.execute() + mrB.run()


# Inline already worked (receiver peel).
def use_ternary_inline_mr(c, ba: BoxA, bb: BoxB):
    return (ba.get() if c else ba.get()).execute() + (bb.get() if c else bb.get()).run()


# Class regression — already worked.
def use_ternary_assign_class(c):
    classA = A() if c else A()
    classB = B() if c else B()
    return classA.execute() + classB.run()


def use_preserves_b(c, bb: BoxB):
    mrB = bb.get() if c else bb.get()
    return mrB.run()
