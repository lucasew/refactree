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


# Class regression — already solid.
def use_match_list_class() -> int:
    n = 0
    match [A()]:
        case [x]:
            n += x.execute()
    match [B()]:
        case [y]:
            n += y.run()
    return n


def use_match_tuple_class() -> int:
    n = 0
    match (A(),):
        case (x,):
            n += x.execute()
    match (B(),):
        case (y,):
            n += y.run()
    return n


def use_match_dict_class() -> int:
    n = 0
    match {"k": A()}:
        case {"k": x}:
            n += x.execute()
    match {"k": B()}:
        case {"k": y}:
            n += y.run()
    return n


def use_match_list_guard_class() -> int:
    n = 0
    match [A()]:
        case [x] if True:
            n += x.execute()
    match [B()]:
        case [y] if True:
            n += y.run()
    return n


# Method-return match subjects under foreign same-leaf.
def use_match_list_mr(ba: BoxA, bb: BoxB) -> int:
    n = 0
    match [ba.get()]:
        case [x]:
            n += x.execute()
    match [bb.get()]:
        case [y]:
            n += y.run()
    return n


def use_match_tuple_mr(ba: BoxA, bb: BoxB) -> int:
    n = 0
    match (ba.get(),):
        case (x,):
            n += x.execute()
    match (bb.get(),):
        case (y,):
            n += y.run()
    return n


def use_match_dict_mr(ba: BoxA, bb: BoxB) -> int:
    n = 0
    match {"k": ba.get()}:
        case {"k": x}:
            n += x.execute()
    match {"k": bb.get()}:
        case {"k": y}:
            n += y.run()
    return n


def use_match_list_guard_mr(ba: BoxA, bb: BoxB) -> int:
    n = 0
    match [ba.get()]:
        case [x] if True:
            n += x.execute()
    match [bb.get()]:
        case [y] if True:
            n += y.run()
    return n


# Assigned subjects already solid — regression for elemOf path.
def use_match_list_assign_mr(ba: BoxA, bb: BoxB) -> int:
    n = 0
    xs = [ba.get()]
    ys = [bb.get()]
    match xs:
        case [x]:
            n += x.execute()
    match ys:
        case [y]:
            n += y.run()
    return n


def use_match_dict_assign_mr(ba: BoxA, bb: BoxB) -> int:
    n = 0
    da = {"k": ba.get()}
    db = {"k": bb.get()}
    match da:
        case {"k": x}:
            n += x.execute()
    match db:
        case {"k": y}:
            n += y.run()
    return n
