class Box:
    helper: int
    stay: int

    def __init__(self, helper, stay):
        self.helper = helper
        self.stay = stay


def use(x):
    match x:
        case Box(helper=h, stay=s):
            return h + s
        case _:
            return 0
