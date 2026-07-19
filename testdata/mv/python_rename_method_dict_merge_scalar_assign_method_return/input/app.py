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


# Scalar-from-merge subscript/get assign method-return under foreign same-leaf.
def use_merge_sub_assign_mr(ba: BoxA, bb: BoxB):
    mrA = ({"k": ba.get()} | {})["k"]
    mrB = ({"k": bb.get()} | {})["k"]
    return mrA.run() + mrB.run()


def use_merge_get_assign_mr(ba: BoxA, bb: BoxB):
    mrGA = ({"k": ba.get()} | {}).get("k")
    mrGB = ({"k": bb.get()} | {}).get("k")
    return mrGA.run() + mrGB.run()


def use_merge_rhs_sub_assign_mr(ba: BoxA, bb: BoxB):
    mrRA = ({} | {"k": ba.get()})["k"]
    mrRB = ({} | {"k": bb.get()})["k"]
    return mrRA.run() + mrRB.run()


def use_merge_rhs_get_assign_mr(ba: BoxA, bb: BoxB):
    mrRGA = ({} | {"k": ba.get()}).get("k")
    mrRGB = ({} | {"k": bb.get()}).get("k")
    return mrRGA.run() + mrRGB.run()


# Inline / da-assign already worked.
def use_merge_inline_mr(ba: BoxA, bb: BoxB):
    return ({"k": ba.get()} | {})["k"].run() + ({"k": bb.get()} | {})["k"].run()


def use_merge_da_assign_mr(ba: BoxA, bb: BoxB):
    da = {"k": ba.get()} | {}
    db = {"k": bb.get()} | {}
    return da["k"].run() + db["k"].run()


# Class regression — already worked.
def use_merge_sub_assign_class():
    classA = ({"k": A()} | {})["k"]
    classB = ({"k": B()} | {})["k"]
    return classA.run() + classB.run()


def use_merge_get_assign_class():
    classGA = ({"k": A()} | {}).get("k")
    classGB = ({"k": B()} | {}).get("k")
    return classGA.run() + classGB.run()


def use_preserves_b(bb: BoxB):
    mrB = ({"k": bb.get()} | {})["k"]
    mrGB = ({"k": bb.get()} | {}).get("k")
    return mrB.run() + mrGB.run()
