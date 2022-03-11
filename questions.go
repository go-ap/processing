package processing

import (
	pub "github.com/go-ap/activitypub"
	s "github.com/go-ap/storage"
)

// QuestionActivity processes matching activities
//
// https://www.w3.org/TR/activitystreams-vocabulary/#h-motivations-questions
//
// The Questions use case primarily deals with representing inquiries of any type.
// See 5.4 Representing Questions for more information: https://www.w3.org/TR/activitystreams-vocabulary/#questions
func QuestionActivity(l s.WriteStore, q *pub.Question) (*pub.Question, error) {
	// NOTE(marius): this behaviour is not in accordance to the spec.
	// Saving the question items as individual objects makes sense to me,
	// but it's not mention in either of the documents we're basing this implementation on.
	// I can't think of any reason why saving them might be a _bad_ idea, so for the moment I'll leave it like this.
	if q.AnyOf != nil {
		return q, pub.OnItemCollection(q.AnyOf, saveQuestionAnswers(l, q))
	}
	if q.OneOf != nil {
		return q, pub.OnItemCollection(q.OneOf, saveQuestionAnswers(l, q))
	}
	return q, nil
}

func saveQuestionAnswers(l s.WriteStore, q *pub.Question) func(col *pub.ItemCollection) error {
	return func(col *pub.ItemCollection) error {
		var err error
		for _, ans := range col.Collection() {
			if iri := ans.GetLink(); len(iri) == 0 {
				SetID(ans, nil, q.Actor)
			}

			pub.OnActivity(q, func(act *pub.Activity) error {
				return updateCreateActivityObject(l, ans, act)
			})

			ans, err = l.Save(ans)
		}
		return err
	}
}
