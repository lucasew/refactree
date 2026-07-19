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


# Dict subscript assign method-return under foreign same-leaf.
def use_dict_sub_assign_mr(ba: BoxA, bb: BoxB):
    da_m = {}
    db_m = {}
    da_m["k"] = ba.get()
    db_m["k"] = bb.get()
    return da_m["k"].execute() + db_m["k"].run()


def use_dict_sub_assign_mr_assign(ba: BoxA, bb: BoxB):
    da_ma = {}
    db_ma = {}
    da_ma["k"] = ba.get()
    db_ma["k"] = bb.get()
    xa = da_ma["k"]
    xb = db_ma["k"]
    return xa.execute() + xb.run()


# List subscript assign method-return under foreign same-leaf.
def use_list_sub_assign_mr(ba: BoxA, bb: BoxB):
    xs_m = [None]
    ys_m = [None]
    xs_m[0] = ba.get()
    ys_m[0] = bb.get()
    return xs_m[0].execute() + ys_m[0].run()


def use_list_sub_assign_mr_assign(ba: BoxA, bb: BoxB):
    xs_ma = [None]
    ys_ma = [None]
    xs_ma[0] = ba.get()
    ys_ma[0] = bb.get()
    xa = xs_ma[0]
    xb = ys_ma[0]
    return xa.execute() + xb.run()


# Class regression — already worked.
def use_dict_sub_assign_class():
    da_c = {}
    db_c = {}
    da_c["k"] = A()
    db_c["k"] = B()
    return da_c["k"].execute() + db_c["k"].run()


def use_list_sub_assign_class():
    xs_c = [None]
    ys_c = [None]
    xs_c[0] = A()
    ys_c[0] = B()
    return xs_c[0].execute() + ys_c[0].run()


def use_preserves_b(bb: BoxB):
    da_pb = {}
    da_pb["k"] = bb.get()
    xs_pb = [None]
    xs_pb[0] = bb.get()
    return da_pb["k"].run() + xs_pb[0].run()
